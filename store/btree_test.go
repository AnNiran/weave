package store

import (
	"bytes"
	"crypto/rand"
	"sort"
	"testing"

	"github.com/iov-one/weave/weavetest/assert"
)

// makeBase returns the base layer
//
// If you want to test a different kvstore implementation
// you can copy most of these tests and change makeBase.
// Once that passes, customize and extend as you wish
func makeBase() CacheableKVStore {
	return MemStore()
}

func assertGetHas(t testing.TB, kv ReadOnlyKVStore, key, val []byte, has bool) {
	t.Helper()
	got, err := kv.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, val, got)
	exists, err := kv.Has(key)
	assert.Nil(t, err)
	assert.Equal(t, has, exists)
}

// TestBTreeCacheGetSet does basic sanity checks on our cache
//
// Other tests should handle deletes, setting same value,
// iterating over ranges, and general fuzzing
func TestBTreeCacheGetSet(t *testing.T) {
	base := makeBase()

	// make sure the btree is empty at start but returns results
	// that are written to it
	k, v := []byte("french"), []byte("fry")
	assertGetHas(t, base, k, nil, false)
	err := base.Set(k, v)
	assert.Nil(t, err)
	assertGetHas(t, base, k, v, true)

	// now layer another btree on top and make sure that we get
	// base data
	cache := base.CacheWrap()
	assertGetHas(t, cache, k, v, true)

	// writing more data is only visible in the cache
	k2, v2 := []byte("LA"), []byte("Dodgers")
	assertGetHas(t, cache, k2, nil, false)
	err = cache.Set(k2, v2)
	assert.Nil(t, err)
	assertGetHas(t, cache, k2, v2, true)
	assertGetHas(t, base, k2, nil, false)

	// we can write the cache to the base layer...
	err = cache.Write()
	assert.Nil(t, err)
	assertGetHas(t, base, k, v, true)
	assertGetHas(t, base, k2, v2, true)

	// we can discard one
	k3, v3 := []byte("Bayern"), []byte("Munich")
	c2 := base.CacheWrap()
	assertGetHas(t, c2, k, v, true)
	assertGetHas(t, c2, k2, v2, true)
	err = c2.Set(k3, v3)
	assert.Nil(t, err)
	c2.Discard()

	// and commit another
	c3 := base.CacheWrap()
	assertGetHas(t, c3, k, v, true)
	assertGetHas(t, c3, k2, v2, true)
	err = c3.Delete(k)
	assert.Nil(t, err)
	err = c3.Write()
	assert.Nil(t, err)

	// make sure it commits proper
	assertGetHas(t, c2, k, nil, false)
	assertGetHas(t, c2, k2, v2, true)
	assertGetHas(t, c2, k3, nil, false)
}

// TestBTreeCacheConflicts checks that we can handle
// overwriting values and deleting underlying values
func TestBTreeCacheConflicts(t *testing.T) {
	// make 10 keys and 20 values....
	ks := randKeys(10, 16)
	vs := randKeys(20, 40)

	cases := map[string]struct {
		parentOps     []Op
		childOps      []Op
		parentQueries []Model // Key is what we query, Value is what we expect
		childQueries  []Model // Key is what we query, Value is what we expect
	}{
		"overwrite one, delete another, add a third": {
			parentOps:     []Op{SetOp(ks[1], vs[1]), SetOp(ks[2], vs[2])},
			childOps:      []Op{SetOp(ks[1], vs[11]), SetOp(ks[3], vs[7]), DelOp(ks[2])},
			parentQueries: []Model{Pair(ks[1], vs[1]), Pair(ks[2], vs[2]), Pair(ks[3], nil)},
			childQueries:  []Model{Pair(ks[1], vs[11]), Pair(ks[2], nil), Pair(ks[3], vs[7])},
		},
	}

	for testName, tc := range cases {
		t.Run(testName, func(t *testing.T) {
			parent := makeBase()
			for _, op := range tc.parentOps {
				op.Apply(parent)
			}

			child := parent.CacheWrap()
			for _, op := range tc.childOps {
				op.Apply(child)
			}

			// now check the parent is unaffected
			for _, q := range tc.parentQueries {
				assertGetHas(t, parent, q.Key, q.Value, q.Value != nil)
			}

			// the child shows changes
			for _, q := range tc.childQueries {
				assertGetHas(t, child, q.Key, q.Value, q.Value != nil)
			}

			// write child to parent and make sure it also shows proper data
			child.Write()
			for _, q := range tc.childQueries {
				assertGetHas(t, parent, q.Key, q.Value, q.Value != nil)
			}
		})
	}
}

// TestFuzzBTreeCacheIterator makes sure the basic iterator
// works. Includes random deletes, but not nested iterators.
func TestFuzzBTreeCacheIterator(t *testing.T) {
	const Size = 50
	const DeleteCount = 20

	toSet := randModels(Size, 8, 40)
	toDel := randModels(DeleteCount, 8, 40)
	expect := sortModels(toSet)
	ops := append(
		makeSetOps(toSet...),
		makeDelOps(toDel...)...)

	parentSet := randModels(Size, 8, 40)
	parentDel := randModels(DeleteCount, 8, 40)
	parentOps := append(
		makeSetOps(parentSet...),
		makeDelOps(parentDel...)...)

	both := sortModels(append(toSet, parentSet...))

	cases := map[string]iterCase{
		"just write to a child with empty parent": {
			pre:   nil,
			child: ops,
			queries: []rangeQuery{
				// forward: no, start, finish, both limits
				{nil, nil, false, expect},
				{expect[10].Key, nil, false, expect[10:]},
				{nil, expect[Size-8].Key, false, expect[:Size-8]},
				{expect[17].Key, expect[28].Key, false, expect[17:28]},

				// reverse: no, start, finish, both limits
				{nil, nil, true, reverse(expect)},
				{expect[34].Key, nil, true, reverse(expect[34:])},
				{nil, expect[19].Key, true, reverse(expect[:19])},
				{expect[6].Key, expect[26].Key, true, reverse(expect[6:26])},
			},
		},
		"iterator combines child and parent": {
			pre:   parentOps,
			child: ops,
			queries: []rangeQuery{
				// forward: no, start, finish, both limits
				{nil, nil, false, both},
				{both[10].Key, nil, false, both[10:]},
				{nil, both[Size-8].Key, false, both[:Size-8]},
				{both[17].Key, both[28].Key, false, both[17:28]},

				// reverse: no, start, finish, both limits
				{nil, nil, true, reverse(both)},
				{both[34].Key, nil, true, reverse(both[34:])},
				{nil, both[19].Key, true, reverse(both[:19])},
				{both[6].Key, both[26].Key, true, reverse(both[6:26])},
			},
		},
	}

	for testName, tc := range cases {
		t.Run(testName, func(t *testing.T) {
			base := makeBase()
			tc.verify(t, base)
		})
	}
}

// TestConflictBTreeCacheIterator makes sure the basic iterator
// works. Includes random deletes, but not nested iterators.
func TestConflictBTreeCacheIterator(t *testing.T) {
	const Size = 50
	const DeleteCount = 20

	ms := randModels(6, 20, 100)
	a, a2, b, b2, c, d := ms[0], ms[1], ms[2], ms[3], ms[4], ms[5]
	// a2, b2 have same keys, different values
	a2.Key = a.Key
	b2.Key = b.Key

	// toSet := randModels(Size, 8, 40)
	// toDel := randModels(DeleteCount, 8, 40)
	// expect := sortModels(toSet)
	// ops := append(
	// 	makeSetOps(toSet),
	// 	makeDelOps(toDel)...)

	// parentSet := randModels(Size, 8, 40)
	// parentDel := randModels(DeleteCount, 8, 40)
	// parentOps := append(
	// 	makeSetOps(parentSet),
	// 	makeDelOps(parentDel)...)

	// both := sortModels(append(toSet, parentSet...))

	expect0 := sortModels([]Model{a2, b2, c, d})
	expect1 := []Model{c}

	cases := map[string]iterCase{
		"overwrite data should show child data": {
			pre:   makeSetOps(a, b, c),
			child: makeSetOps(a2, b2, d),
			queries: []rangeQuery{
				// query for the values in child
				{nil, nil, false, expect0},
				{expect0[1].Key, expect0[3].Key, false, expect0[1:3]},

				{nil, nil, true, reverse(expect0)},
			},
		},
		"overwrite data should show child data 2": {
			pre:   makeSetOps(a, c, d),
			child: makeDelOps(a, b, d),
			queries: []rangeQuery{
				// query all should find just one, skip delete
				{nil, nil, false, expect1},
				// query cuts off at actual value, should be empty
				{nil, c.Key, false, nil},
			},
		},
	}

	for testName, tc := range cases {
		t.Run(testName, func(t *testing.T) {
			base := makeBase()
			tc.verify(t, base)
		})
	}
}

func randBytes(length int) []byte {
	res := make([]byte, length)
	rand.Read(res)
	return res
}

// randKeys returns a slice of count keys, all of a given size
func randKeys(count, size int) [][]byte {
	res := make([][]byte, count)
	for i := 0; i < count; i++ {
		res[i] = randBytes(size)
	}
	return res
}

// randModels produces a random set of models
func randModels(count, keySize, valueSize int) []Model {
	models := make([]Model, count)
	for i := 0; i < count; i++ {
		models[i].Key = randBytes(keySize)
		models[i].Value = randBytes(valueSize)
	}
	return models
}

// iterCase is a test case for iteration
type iterCase struct {
	pre     []Op
	child   []Op
	queries []rangeQuery
}

func (i iterCase) verify(t testing.TB, base CacheableKVStore) {
	for _, op := range i.pre {
		op.Apply(base)
	}

	child := base.CacheWrap()
	for _, op := range i.child {
		op.Apply(base)
	}

	for _, q := range i.queries {
		var iter Iterator
		var err error
		if q.reverse {
			iter, err = child.ReverseIterator(q.start, q.end)
		} else {
			iter, err = child.Iterator(q.start, q.end)
		}
		assert.Nil(t, err)
		// Make sure proper iteration works.
		for i := 0; i < len(q.expected); i++ {
			assert.Equal(t, iter.Valid(), true)
			assert.Equal(t, q.expected[i].Key, iter.Key())
			assert.Equal(t, q.expected[i].Value, iter.Value())
			iter.Next()
		}
		assert.Equal(t, iter.Valid(), false)
		iter.Close()
	}
}

// range query checks the results of iteration
type rangeQuery struct {
	start    []byte
	end      []byte
	reverse  bool
	expected []Model
}

// reverse returns a copy of the slice with elements in reverse order
func reverse(models []Model) []Model {
	max := len(models)
	res := make([]Model, max)
	for i := 0; i < max; i++ {
		res[i] = models[max-1-i]
	}
	return res
}

// sortModels returns a copy of the models sorted by key
func sortModels(models []Model) []Model {
	res := make([]Model, len(models))
	copy(res, models)
	// sort by key
	sort.Slice(res, func(i, j int) bool {
		return bytes.Compare(res[i].Key, res[j].Key) < 0
	})
	return res
}

func makeSetOps(ms ...Model) []Op {
	res := make([]Op, len(ms))
	for i, m := range ms {
		res[i] = SetOp(m.Key, m.Value)
	}
	return res
}

func makeDelOps(ms ...Model) []Op {
	res := make([]Op, len(ms))
	for i, m := range ms {
		res[i] = DelOp(m.Key)
	}
	return res
}
