package dbx

import (
	"github.com/peralta/go-observability-kit/bootstrap"
	"gorm.io/gorm"
)

// Option is reserved for DB instrumentation options in later phases.
type Option func(*options)

type options struct{}

// WrapGORM returns the same DB handle for skeleton compatibility.
func WrapGORM(db *gorm.DB, _ *bootstrap.Runtime, _ ...Option) *gorm.DB {
	return db
}
