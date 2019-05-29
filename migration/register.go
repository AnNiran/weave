package migration

import (
	"reflect"

	"github.com/iov-one/weave"
	"github.com/iov-one/weave/errors"
)

type Migratable interface {
	// GetMetadata returns a metadata information about given entity. This
	// method is generated by the protobuf compiler for every message that
	// has a metadata field.
	GetMetadata() *weave.Metadata

	// Validate returns an error instance state is not valid. This method
	// is implemented by all models and messages.
	Validate() error
}

// Migrator is a function that migrates in place an entity of a single type.
type Migrator func(weave.ReadOnlyKVStore, Migratable) error

// NoModification is a migration function that migrates data that requires no
// change. It should be used to register migrations that do not require any
// modifications.
func NoModification(weave.ReadOnlyKVStore, Migratable) error {
	return nil
}

// RefuseMigration is a migration function that always fails. Its use is
// expected when there is no migration path to given version. This is accepted
// migration callback function for messages but should be avoided for models.
func RefuseMigration(weave.ReadOnlyKVStore, Migratable) error {
	return errors.Wrap(errors.ErrSchema, "no migration path from given schema version")
}

func newRegister() *register {
	return &register{
		migrateTo: make(map[payloadVersion]Migrator),
	}
}

type register struct {
	migrateTo map[payloadVersion]Migrator
}

// payloadVersion references a message or a model at a given schema version.
type payloadVersion struct {
	payload reflect.Type
	version uint32
}

func (r *register) MustRegister(migrationTo uint32, msgOrModel Migratable, fn Migrator) {
	if err := r.Register(migrationTo, msgOrModel, fn); err != nil {
		panic(err)
	}
}

func (r *register) Register(migrationTo uint32, msgOrModel Migratable, fn Migrator) error {
	if migrationTo < 1 {
		return errors.Wrap(errors.ErrInput, "minimal allowed version is 1")
	}

	tp := reflect.TypeOf(msgOrModel)

	if migrationTo > 1 {
		prev := payloadVersion{
			version: migrationTo - 1,
			payload: tp,
		}
		if _, ok := r.migrateTo[prev]; !ok {
			return errors.Wrapf(errors.ErrInput, "missing %d version migration", prev.version)
		}
	}

	pv := payloadVersion{
		version: migrationTo,
		payload: tp,
	}
	if _, ok := r.migrateTo[pv]; ok {
		return errors.Wrapf(errors.ErrDuplicate,
			"already registered: %s.%s:%d", tp.PkgPath(), tp.Name(), migrationTo)
	}
	r.migrateTo[pv] = fn
	return nil
}

// Apply updates the object by applying all missing data migrations. Even a no
// modification migration is updating the metadata to point to the latest data
// format version.
//
// Because changes are applied directly on the passed object (in place), even
// if this function fails some of the data migrations might be applied.
//
// A valid object metadata must contain a schema version greater than zero.
// Not migrated object (initial state) is always having a metadata schema value
// set to 1.
//
// Validation method is called only on the final version of the object.
func (r *register) Apply(db weave.ReadOnlyKVStore, m Migratable, migrateTo uint32) error {
	if migrateTo < 1 {
		return errors.Wrap(errors.ErrInput, "minimal allowed version is 1")
	}

	meta := m.GetMetadata()
	if err := meta.Validate(); err != nil {
		return err
	}

	tp := reflect.TypeOf(m)
	for v := meta.Schema + 1; v <= migrateTo; v++ {
		migrate, ok := r.migrateTo[payloadVersion{payload: tp, version: v}]
		if !ok {
			return errors.Wrapf(errors.ErrSchema, "migration to version %d missing", v)
		}
		if err := migrate(db, m); err != nil {
			return errors.Wrapf(err, "migration to version %d", v)
		}
		meta.Schema = v
	}

	if err := m.Validate(); err != nil {
		return errors.Wrap(err, "validation")
	}
	return nil
}

// reg is a globally available register instance that must be used during the
// runtime to register migration handlers.
// Register is declared as a separate type so that it can be tested without
// worrying about the global state.
var reg *register = newRegister()

// MustRegister registers a migration function for a given message or model.
// Migration function will be called when migrating data from a version one
// less than migrationTo value.
// Minimal allowed migrationTo version is 1. Version upgrades for each type
// must be registered in sequential order.
func MustRegister(migrationTo uint32, msgOrModel Migratable, fn Migrator) {
	reg.MustRegister(migrationTo, msgOrModel, fn)
}
