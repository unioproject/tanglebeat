package utils

type ErrorCounter interface {
	CheckError(endpoint string, err error) bool
}

type DummyAEC struct{}

func (*DummyAEC) CheckError(endpoint string, err error) bool {
	return err != nil
}
