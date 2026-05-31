package matchers

import "github.com/sirupsen/logrus"

// AlertGeneratedMatcher is an interface that every integration should implement to verify whether an expected
// security alert was created
type AlertGeneratedMatcher interface {
	// HasExpectedAlert verifies on a third-party service whether an alert was properly generated for the given detonation UUID
	HasExpectedAlert(indicators []string, logger *logrus.Entry) (bool, error)

	// String returns the textual, user-friendly representation of the matcher
	String() string

	// Cleanup closes the generated alerts of a given detonation on a third-party service
	Cleanup(indicators []string, logger *logrus.Entry) error

	// MatcherName returns the name of the matcher
	MatcherName() string

	// AlertName returns the name of the alert being matched
	AlertName() string
}
