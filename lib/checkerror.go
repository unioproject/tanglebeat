package lib

type ErrorCounter interface {
	CheckError(endpoint string, err error) bool
}
