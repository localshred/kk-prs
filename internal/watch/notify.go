package watch

import "github.com/gen2brain/beeep"

type beeepNotifier struct{}

// NewBeeepNotifier returns a Notifier that uses beeep for desktop notifications.
func NewBeeepNotifier() Notifier { return beeepNotifier{} }

// Notify is best-effort: a desktop-notification failure never fails a tick.
func (beeepNotifier) Notify(key, title, message, url string) error {
	_ = beeep.Notify(title+" ("+key+")", message, "")
	return nil
}
