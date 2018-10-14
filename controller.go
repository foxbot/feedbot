package feedbot

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3" // driver for database/sql
	"github.com/pkg/errors"
)

const schema string = `
CREATE TABLE feeds (
	id int PRIMARY KEY,
	uri text UNIQUE NOT NULL,
	last_updated text NOT NULL,
);

CREATE TABLE guild_config (
	id text PRIMARY KEY,
	contact text NOT NULL,
	enable_embeds int NOT NULL,
	enable_webhooks int NOT NULL,
);

CREATE TABLE subscriptions (
	id int PRIMARY KEY,
	channel_id text NOT NULL,
	feed_id int NOT NULL,

	FOREIGN KEY(feed_id) REFERENCES feeds(id)
);

CREATE TABLE subscription_overrides (
	id int PRIMARY KEY,
	sub_id int NOT NULL,
	enable_embeds int,
	enable_webhooks int,

	FOREIGN KEY(sub_id) REFERENCES subscriptions(id) ON DELETE CASCADE
);
`

// Feed contains the ID and URI of a RSS feed in the database
type Feed struct {
	ID          int
	URI         string
	LastUpdated time.Time
}

// Subscription contains the metadata for a subscription to a feed
type Subscription struct {
	ID        int
	ChannelID string
	FeedID    int
}

// GuildConfig contains guild-wide configuration
type GuildConfig struct {
	ID       string
	Contact  string
	Embeds   bool
	Webhooks bool
}

// Overwrite contains a subscription overwrite
type Overwrite struct {
	ID             int
	SubscriptionID int
	Embeds         bool
	Webhooks       bool
}

// Controller contains logic for manipulating the database
type Controller struct {
	db *sql.DB
}

var (
	// ErrSubExists is returned when a subscription already exists for a feed/channel
	ErrSubExists = errors.New("a subscription already exists")
)

// NewController creates a new controller
func NewController() (*Controller, error) {
	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON;")
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
		return errors.WithStack(err)
	}
	return nil
}

// GetOrCreateFeed will insert a new RSS Feed to the database if one does not exist, and return
// a Feed for it.
func (c *Controller) GetOrCreateFeed(uri string) (*Feed, error) {
	r, err := c.db.Query(`
	INSERT OR IGNORE INTO feeds (uri, last_updated) VALUES ($1, $2);
	SELECT (id, uri, last_updated) FROM feeds WHERE uri = $1;
	`, uri, "1970-01-01 00:00:00")

	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer r.Close()

	var f Feed
	err = r.Scan(&f.ID, &f.URI, &f.LastUpdated)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &f, nil
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
			return f, errors.WithStack(err)
		}
		f = append(f, i)
	}

	return f, nil
}

// UpdateFeedTimestamp updates a feed's last_updated value
func (c *Controller) UpdateFeedTimestamp(feed *Feed, timestamp *time.Time) error {
	r, err := c.db.Exec("UPDATE feeds SET last_updated = ? WHERE id = ?;",
		timestamp, feed.ID)
	if err != nil {
		return errors.WithStack(err)
	}

	if n, err := r.RowsAffected(); err != nil {
		return errors.WithStack(err)
	} else if n != 1 {
		return errors.New("invalid number of rows affected")
	}

	return nil
}

// AddSubscription adds a subscription to the given feed for a channel
func (c *Controller) AddSubscription(channelID string, feedID int) (*Subscription, error) {
	// ensure subscriptions don't already exist
	r, err := c.db.Query(`
	SELECT (id) FROM subscriptions WHERE feed_id = ?, channel_id = ?
	`, feedID, channelID)
	defer r.Close()

	if err != nil && err != sql.ErrNoRows {
		return nil, errors.WithStack(err)
	}
	if err == nil {
		var s Subscription
		err = r.Scan(&s.ID)
		if err != nil {
			return nil, err
		}
		s.ChannelID = channelID
		s.FeedID = feedID
		return &s, ErrSubExists
	}

	r, err = c.db.Query(`
	INSERT INTO subscriptions (channel_id, feed_id)
	VALUES ($1, $2);
	SELECT (id) FROM subscriptions WHERE channel_id = $1, feed_id = $2;
	`, channelID, feedID)
	defer r.Close()

	if err != nil {
		return nil, errors.WithStack(err)
	}
	var s Subscription
	err = r.Scan(&s.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	s.ChannelID = channelID
	s.FeedID = feedID
	return &s, nil
}

// GetSubscription gets a subscription from its ID
func (c *Controller) GetSubscription(id int) (*Subscription, error) {
	r, err := c.db.Query("SELECT (id, channel_id, feed_id) FROM subscriptions WHERE id = ?;")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var s Subscription
	err = r.Scan(&s.ID, &s.ChannelID, &s.FeedID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &s, nil
}

// ModifySubscriptionChannel changes the channel_id for a Subscription
func (c *Controller) ModifySubscriptionChannel(id int, channelID string) error {
	r, err := c.db.Exec("UPDATE subscriptions SET channel_id = ? WHERE id = ?;", channelID, id)
	if err != nil {
		return errors.WithStack(err)
	}
	if n, err := r.RowsAffected(); err == nil {
		if n == 0 {
			return sql.ErrNoRows
		}
	}
	return err
}

// DestroySubscription deletes a subscription from the database
func (c *Controller) DestroySubscription(id int) error {
	r, err := c.db.Exec("DELETE FROM subscriptions WHERE id = ?;", id)
	if err != nil {
		return errors.WithStack(err)
	}
	if n, err := r.RowsAffected(); err == nil {
		if n == 0 {
			return errors.Wrap(sql.ErrNoRows, "no rows on subscription delete")
		}
	}
	return err
}

// ModifyGuildContact changes the guild's contact address
func (c *Controller) ModifyGuildContact(guildID string, contact string) error {
	r, err := c.db.Exec("UPDATE guild_settings SET contact = ? WHERE id = ?;", contact, guildID)
	if err != nil {
		return errors.WithStack(err)
	}
	if n, err := r.RowsAffected(); err == nil {
		if n == 0 {
			return sql.ErrNoRows
		}
	}
	return err
}
