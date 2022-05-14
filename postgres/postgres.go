package postgres

import (
	"database/sql"

	"github.com/jmoiron/sqlx"

	_ "github.com/lib/pq" //nolint:golint
	"github.com/pkg/errors"
)

type DbConn interface {
	Master() *sql.DB
	Slave() *sql.DB
	PingMaster() bool
	PingSlave() bool
}

// New инициализация подключений к мастер и слейву базы данных postgres
func New(masterDSN string, slaveDSN string, maxOpenClients int) (*DBConnections, error) {
	master, err := sql.Open("postgres", masterDSN)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	slave, err := sql.Open("postgres", slaveDSN)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	db := &DBConnections{
		master: master,
		slave:  slave,
	}

	// Проверяем подключения к базам данных, но при ошибках не выходим сразу, а только логируем.
	// Мы выведем эти ошибки в /health/check, и уже снаружи будут решать, что с ними делать.
	// Обычно скрипты выкатки не возвращают ногу под нагрузку, если её /health/check выдаёт ошибку
	// хотя бы по одному ресурсу; но легко себе представить, что во время какой-нибудь аварии нам понадобится,
	// например, перезапустить сервис при недоступном мастере БД: ведь читающие запросы он сможет обслуживать.
	db.PingMaster()
	db.PingSlave()
	db.master.SetMaxOpenConns(maxOpenClients)
	db.slave.SetMaxOpenConns(maxOpenClients)
	return db, nil
}

type DBConnections struct {
	master *sql.DB
	slave  *sql.DB
}

// Master реализация интерфейса
func (db *DBConnections) Master() *sql.DB {
	return db.master
}

// Slave реализация интерфейса
func (db *DBConnections) Slave() *sql.DB {
	return db.slave
}

// MasterX реализация интерфейса
func (db *DBConnections) MasterX() *sqlx.DB {
	return sqlx.NewDb(db.master, "postgres")
}

// SlaveX реализация интерфейса
func (db *DBConnections) SlaveX() *sqlx.DB {
	return sqlx.NewDb(db.slave, "postgres")
}

func (db *DBConnections) PingMaster() bool {
	if err := db.master.Ping(); err != nil {
		return false
	}
	return true
}

func (db *DBConnections) PingSlave() bool {
	if err := db.slave.Ping(); err != nil {
		return false
	}
	return true
}
