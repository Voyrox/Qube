package database

import (
	"fmt"
	"time"

	"github.com/Voyrox/Qube/hub/core/config"
	"github.com/gocql/gocql"
)

type ScyllaDB struct {
	session  *gocql.Session
	keyspace string
}

func NewScyllaDB(cfg *config.Config) (*ScyllaDB, error) {
	cluster := gocql.NewCluster(cfg.ScyllaHosts...)
	cluster.Consistency = gocql.Quorum
	cluster.ProtoVersion = 4
	cluster.ConnectTimeout = time.Second * 10
	cluster.Timeout = time.Second * 10

	if cfg.ScyllaUsername != "" && cfg.ScyllaPassword != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: cfg.ScyllaUsername,
			Password: cfg.ScyllaPassword,
		}
	}

	// First, connect without keyspace to create it if needed
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ScyllaDB: %w", err)
	}

	// Create keyspace if it doesn't exist
	keyspaceQuery := fmt.Sprintf(`
		CREATE KEYSPACE IF NOT EXISTS %s
		WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}
	`, cfg.ScyllaKeyspace)

	if err := session.Query(keyspaceQuery).Exec(); err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to create keyspace: %w", err)
	}

	session.Close()

	// Reconnect with the keyspace
	cluster.Keyspace = cfg.ScyllaKeyspace
	session, err = cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to keyspace: %w", err)
	}

	return &ScyllaDB{
		session:  session,
		keyspace: cfg.ScyllaKeyspace,
	}, nil
}

func (db *ScyllaDB) Close() {
	if db.session != nil {
		db.session.Close()
	}
}

func (db *ScyllaDB) Session() *gocql.Session {
	return db.session
}

func (db *ScyllaDB) InitSchema() error {
	queries := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			username TEXT,
			email TEXT,
			password_hash TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS ON users (username)`,
		`CREATE INDEX IF NOT EXISTS ON users (email)`,

		// Images table
		`CREATE TABLE IF NOT EXISTS images (
			id UUID PRIMARY KEY,
			name TEXT,
			tag TEXT,
			owner_id UUID,
			description TEXT,
			digest TEXT,
			size BIGINT,
			downloads BIGINT,
			pulls BIGINT,
			stars BIGINT,
			is_public BOOLEAN,
			file_path TEXT,
			logo_path TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			last_updated TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS ON images (name)`,
		`CREATE INDEX IF NOT EXISTS ON images (owner_id)`,

		// Image tags table (for multiple tags per image)
		`CREATE TABLE IF NOT EXISTS image_tags (
			image_id UUID,
			tag TEXT,
			created_at TIMESTAMP,
			PRIMARY KEY (image_id, tag)
		)`,

		// Stars/Favorites table
		`CREATE TABLE IF NOT EXISTS stars (
			user_id UUID,
			image_id UUID,
			created_at TIMESTAMP,
			PRIMARY KEY (user_id, image_id)
		)`,
		`CREATE INDEX IF NOT EXISTS ON stars (image_id)`,

		// Downloads counter table
		`CREATE TABLE IF NOT EXISTS image_downloads (
			image_id UUID,
			downloads COUNTER,
			PRIMARY KEY (image_id)
		)`,

		// Comments table
		`CREATE TABLE IF NOT EXISTS comments (
			id UUID,
			image_id UUID,
			user_id UUID,
			content TEXT,
			created_at TIMESTAMP,
			PRIMARY KEY (image_id, created_at, id)
		) WITH CLUSTERING ORDER BY (created_at DESC)`,
	}

	for _, query := range queries {
		if err := db.session.Query(query).Exec(); err != nil {
			return fmt.Errorf("failed to execute query: %w\nQuery: %s", err, query)
		}
	}

	return nil
}
