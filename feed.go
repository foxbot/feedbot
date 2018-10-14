package feedbot

import (
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

func (f *FeedChecker) checkOnce() error {
	feeds, err := f.controller.GetFeeds()
	if err != nil {
		return errors.Wrap(err, "couldn't retrieve feeds")
	}

	fp := gofeed.NewParser()

	var errs []error
	for _, dbFeed := range feeds {
		feed, err := fp.ParseURL(dbFeed.URI)

		// don't halt all progress because one feed bounced a 404 back
		if err != nil {
			errs = append(errs, err)
			continue
		}

		
	}

	return nil
}
