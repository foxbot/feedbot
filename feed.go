package feedbot

import (
	"fmt"

	"github.com/mmcdole/gofeed"
	"github.com/pkg/errors"
)

// FeedChecker contains the application logic for checking RSS feeds
type FeedChecker struct {
	controller *Controller
}

// NewFeedChecker creates a new FeedChecker
func NewFeedChecker(c *Controller) (*FeedChecker, error) {
	return &FeedChecker{
		controller: c,
	}, nil
}

// Close disposes of the FeedChecker
func (f *FeedChecker) Close() {
}

// checkOnce will loop over all feeds in the database, ping the remote, and check for
// updates.
//
// for each feed, we:
// - check the remote
// - see if any new items have been appended
// - make a list of new items, dispatch those elsewhere to be handled
// - update the database with the new most-recent timestamp
func (f *FeedChecker) checkOnce() []error {
	feeds, err := f.controller.GetFeeds()
	if err != nil {
		return []error{errors.Wrap(err, "couldn't retrieve feeds")}
	}

	fp := gofeed.NewParser()

	var errs []error

feedsLoop:
	for _, dbFeed := range feeds {
		feed, err := fp.ParseURL(dbFeed.URI)

		// don't halt all progress because one feed bounced a 404 back
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if len(feed.Items) == 0 {
			continue
		}

		// use the timestamp of the feed's most recent entry, rather than the feed's updated time.
		// some generators use the timestamp of compilation to mark the feed, rather than its most
		// recent post

		recent := feed.Items[0] // TODO: are RSS feeds always sorted with most-recent at the top?
		if recent.PublishedParsed == nil {
			err = errors.New(fmt.Sprintf("the feed at %s contained an entry with no timestamp!", dbFeed.URI))
			errs = append(errs, err)
			continue
		}

		minTime := dbFeed.LastUpdated.Unix()
		if minTime >= recent.PublishedParsed.Unix() {
			continue
		}

		var items []*gofeed.Item
		for _, item := range feed.Items {
			if item.PublishedParsed == nil {
				err = errors.New(fmt.Sprintf("the feed at %s contained an entry with no timestamp!", dbFeed.URI))
				errs = append(errs, err)
				continue feedsLoop
			}
			if minTime >= item.PublishedParsed.Unix() {
				break
			}
			items = append(items, item)
		}

		// TODO: send these off to Discord
		fmt.Printf("handled %d new items for feed %s!", len(items), dbFeed.URI)

		if err = f.controller.UpdateFeedTimestamp(&dbFeed, recent.PublishedParsed); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
