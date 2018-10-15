package feedbot

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3" // driver for database/sql
	"github.com/pkg/errors"
)

const schema string = `
CREATE TABLE feeds (
	id INTEGER PRIMARY KEY,
	uri text UNIQUE NOT NULL,
	last_updated timestamp NOT NULL
);

CREATE TABLE guild_config (
	id text PRIMARY KEY,
	contact text NOT NULL,
	enable_embeds int NOT NULL,
	enable_webhooks int NOT NULL
);

CREATE TABLE subscriptions (
	id INTEGER PRIMARY KEY,
	guild_id text NOT NULL,
	channel_id text NOT NULL,
	feed_id int NOT NULL,

	FOREIGN KEY(feed_id) REFERENCES feeds(id)
);

CREATE TABLE subscription_overrides (
	id INTEGER PRIMARY KEY,
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
	GuildID   string
	ChannelID string
	FeedID    int
	Feed      *Feed
	Overwrite *Overwrite
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
	Embeds         sql.NullBool
	Webhooks       sql.NullBool
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
	_, err := c.db.Exec(`
	INSERT OR IGNORE INTO feeds (uri, last_updated) VALUES ($1, $2);
	`, uri, time.Time{})

	if err != nil {
		return nil, errors.WithStack(err)
	}

	rs, err := c.db.Query("SELECT id, uri, last_updated FROM feeds WHERE uri = $1;", uri)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer rs.Close()
	rs.Next()

	var f Feed
	err = rs.Scan(&f.ID, &f.URI, &f.LastUpdated)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &f, nil
}

// GetFeeds will get a list of feeds to query from the database
func (c *Controller) GetFeeds() ([]Feed, error) {
	f := []Feed{}
	r, err := c.db.Query("SELECT id, uri, last_updated FROM feeds;")
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
func (c *Controller) AddSubscription(channelID, guildID string, feedID int) (*Subscription, error) {
	// ensure subscriptions don't already exist
	r, err := c.db.Query(`
	SELECT id FROM subscriptions WHERE feed_id = ? AND channel_id = ?;
	`, feedID, channelID)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	defer r.Close()
	if r.Next() {
		var s Subscription
		err = r.Scan(&s.ID)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		s.ChannelID = channelID
		s.FeedID = feedID
		return &s, ErrSubExists
	}

	_, err = c.db.Exec(`
	INSERT INTO subscriptions (guild_id, channel_id, feed_id)
	VALUES (?, ?, ?);
	`, guildID, channelID, feedID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	_, err = c.db.Exec(`
	INSERT INTO subscription_overrides (sub_id) 
		SELECT id FROM subscriptions WHERE channel_id = ? AND feed_id = ?
	`, channelID, feedID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rs, err := c.db.Query("SELECT id FROM subscriptions WHERE channel_id = $1 AND feed_id = $2;",
		channelID, feedID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer rs.Close()
	rs.Next()

	var s Subscription
	err = rs.Scan(&s.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	s.GuildID = guildID
	s.ChannelID = channelID
	s.FeedID = feedID
	return &s, nil
}

// GetSubscription gets a subscription from its ID
func (c *Controller) GetSubscription(id int) (*Subscription, error) {
	r, err := c.db.Query("SELECT id, guild_id, channel_id, feed_id FROM subscriptions WHERE id = ?;", id)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer r.Close()
	r.Next()

	var s Subscription
	err = r.Scan(&s.ID, &s.GuildID, &s.ChannelID, &s.FeedID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &s, nil
}

// GetSubscriptions selects all subscriptions for a given guild
func (c *Controller) GetSubscriptions(guildID string) ([]Subscription, error) {
	var subs []Subscription
	r, err := c.db.Query(`
	SELECT s.id, s.channel_id, f.uri, o.enable_embeds, o.enable_webhooks
		FROM subscriptions as s
		INNER JOIN feeds as f ON f.id = s.feed_id
		INNER JOIN subscription_overrides as o ON o.sub_id = s.id
		WHERE s.guild_id = ?;
	`, guildID)

	if err != nil {
		return subs, errors.WithStack(err)
	}
	defer r.Close()
	for r.Next() {
		var s Subscription
		var f Feed
		var o Overwrite
		err = r.Scan(&s.ID, &s.ChannelID, &f.URI, &o.Embeds, &o.Webhooks)
		if err != nil {
			return subs, errors.WithStack(err)
		}
		s.Feed = &f
		s.Overwrite = &o
		subs = append(subs, s)
	}
	return subs, nil
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

// CreateGuildConfig creates an empty GuildConfig for a guild
func (c *Controller) CreateGuildConfig(guildID string, ownerContact string) error {
	r, err := c.db.Exec(`
	INSERT INTO guild_config (id, contact, enable_embeds, enable_webhooks)
	VALUES (?, ?, 0, 0);
	`, guildID, ownerContact)
	if err != nil {
		return errors.WithStack(err)
	}
	if n, err := r.RowsAffected(); err == nil {
		if n == 0 {
			return errors.Wrap(sql.ErrNoRows, "no rows on modify guild embeds")
		}
	}
	return err
}

// GetGuildConfig gets a guild's config
func (c *Controller) GetGuildConfig(guildID string) (*GuildConfig, error) {
	r, err := c.db.Query(`
	SELECT id, contact, enable_embeds, enable_webhooks
	FROM guild_config WHERE id = ?;
	`, guildID)

	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer r.Close()
	r.Next()

	var g GuildConfig
	err = r.Scan(&g.ID, &g.Contact, &g.Embeds, &g.Webhooks)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &g, nil
}

// ModifyGuildContact changes the guild's contact address
func (c *Controller) ModifyGuildContact(guildID string, contact string) error {
	r, err := c.db.Exec("UPDATE guild_config SET contact = ? WHERE id = ?;", contact, guildID)
	if err != nil {
		return errors.WithStack(err)
	}
	if n, err := r.RowsAffected(); err == nil {
		if n == 0 {
			return errors.Wrap(sql.ErrNoRows, "no rows on modify guild contact")
		}
	}
	return errors.WithStack(err)
}

// ModifyGuildEmbeds changes the guild's embed rule
func (c *Controller) ModifyGuildEmbeds(guildID string, embeds bool) error {
	r, err := c.db.Exec("UPDATE guild_config SET enable_embeds = ? WHERE id = ?;", embeds, guildID)
	if err != nil {
		return errors.WithStack(err)
	}
	if n, err := r.RowsAffected(); err == nil {
		if n == 0 {
			return errors.Wrap(sql.ErrNoRows, "no rows on modify guild embeds")
		}
	}
	return errors.WithStack(err)
}

// ModifyGuildWebhooks changes the guild's webhook rule
func (c *Controller) ModifyGuildWebhooks(guildID string, embeds bool) error {
	r, err := c.db.Exec("UPDATE guild_config SET enable_embeds = ? WHERE id = ?;", embeds, guildID)
	if err != nil {
		return errors.WithStack(err)
	}
	if n, err := r.RowsAffected(); err == nil {
		if n == 0 {
			return errors.Wrap(sql.ErrNoRows, "no rows on modify guild webhooks")
		}
	}
	return errors.WithStack(err)
}

// DestroyGuildData removes all data assosciated with a guild.
func (c *Controller) DestroyGuildData(guildID string) {
	// TODO
}

// ModifyOverwriteEmbeds changes the embeds policy of a subscription overwrite
func (c *Controller) ModifyOverwriteEmbeds(subID int, embeds bool) error {
	r, err := c.db.Exec("UPDATE subscription_overrides SET enable_embeds = ? WHERE sub_id = ?",
		embeds, subID)
	if err != nil {
		return err
	}
	if n, err := r.RowsAffected(); err == nil {
		if n == 0 {
			return errors.Wrap(sql.ErrNoRows, "no rows on modify override embeds")
		}
	}
	return errors.WithStack(err)
}

// ModifyOverwriteWebhooks changes the webhooks policy of a subscription overwrite
func (c *Controller) ModifyOverwriteWebhooks(subID int, webhooks bool) error {
	r, err := c.db.Exec("UPDATE subscription_overrides SET enable_webhooks = ? WHERE sub_id = ?",
		webhooks, subID)
	if err != nil {
		return err
	}
	if n, err := r.RowsAffected(); err == nil {
		if n == 0 {
			return errors.Wrap(sql.ErrNoRows, "no rows on modify override embeds")
		}
	}
	return errors.WithStack(err)
}
