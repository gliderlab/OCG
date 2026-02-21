// Package binddb provides SQLite binary binding functionality.
// This is a stub when binddb build tag is not enabled.
package binddb

// BindDB is a no-op when binddb tag is not set
func BindDB() (string, error) {
	return "", nil
}
