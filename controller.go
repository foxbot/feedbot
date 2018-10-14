package feedbot

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3" // driver for database/sql
)

const schema string = `
CREATE TABLE feeds (
	id int PRIMARY KEY,
	uri text UNIQUE NOT NULL,
	last_entry text NOT NULL,
);
`

// Feed contains the ID and URI of a RSS feed in the database
type Feed struct {
	ID          int
	URI         string
	LastUpdated time.Time
}

// Controller contains logic for manipulating the database
type Controller struct {
	db *sql.DB
}

// NewController creates a new controller
func NewController() (*Controller, error) {
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		return nil, err
	}
	return &Controller{
		db: db,
	}, nil
}

// CreateTables should only be called once; this will initalize the
// database for use with the bot.
func (c *Controller) CreateTables() error {
	_, err := c.db.Exec(schema)
	if err != nil {
		return err
	}
	return nil
}

// GetFeeds will get a list of feeds to query from the database
func (c *Controller) GetFeeds() ([]Feed, error) {
	f := []Feed{}
	r, err := c.db.Query("SELECT (id, uri, last_updated) FROM feeds;")
	if err != nil {
		return f, err
	}
	defer r.Close()

	for r.Next() {
		var i Feed
		if err = r.Scan(&i.ID, &i.URI, &i.LastUpdated); err != nil {
			return f, err
		}
		f = append(f, i)
	}

	return f, nil
}
