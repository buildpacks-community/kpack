package registryfakes

import (
	"fmt"
	"time"
)

func NewFakeRemoteImage(image string, digest string) *FakeRemoteImage {
	return &FakeRemoteImage{
		image:     image,
		digest:    digest,
		labels:    make(map[string]string),
		env:       make(map[string]string),
		createdAt: time.Now(),
	}
}

type FakeRemoteImage struct {
	image     string
	digest    string
	labels    map[string]string
	env       map[string]string
	createdAt time.Time
}

func (f *FakeRemoteImage) CreatedAt() (time.Time, error) {
	return f.createdAt, nil
}

func (f *FakeRemoteImage) Identifier() (string, error) {
	return fmt.Sprintf("%s@%s", f.image, f.digest), nil
}

func (f *FakeRemoteImage) Label(k string) (string, error) {
	return f.labels[k], nil
}

func (f *FakeRemoteImage) Env(k string) (string, error) {
	return f.env[k], nil
}

func (f *FakeRemoteImage) SetLabel(k string, v string) error {
	f.labels[k] = v
	return nil
}

func (i *FakeRemoteImage) SetEnv(k string, v string) error {
	i.env[k] = v
	return nil
}
