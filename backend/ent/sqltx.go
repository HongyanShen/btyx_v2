package ent

import (
	"database/sql"
	"errors"

	entsql "entgo.io/ent/dialect/sql"
)

// SQLTx exposes the current PostgreSQL transaction for River.InsertTx.
// Commit and rollback remain the responsibility of the Ent transaction owner.
func (tx *Tx) SQLTx() (*sql.Tx, error) {
	driver, ok := tx.config.driver.(*txDriver)
	if ok {
		if connection, ok := driver.tx.(*entsql.Tx); ok {
			if native, ok := connection.ExecQuerier.(*sql.Tx); ok {
				return native, nil
			}
		}
	}

	return nil, errors.New("River 需要 Ent 当前使用的 database/sql 事务")
}
