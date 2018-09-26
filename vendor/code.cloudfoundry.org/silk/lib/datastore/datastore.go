package datastore

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/filelock"
	"code.cloudfoundry.org/silk/lib/serial"
)

//go:generate counterfeiter -o ../fakes/file_locker.go --fake-name FileLocker . FileLocker
type FileLocker interface {
	filelock.FileLocker
}

//go:generate counterfeiter -o ../fakes/locked_file.go --fake-name LockedFile . LockedFile
type LockedFile interface {
	filelock.LockedFile
}

//go:generate counterfeiter -o ../fakes/datastore.go --fake-name Datastore . Datastore
type Datastore interface {
	Add(handle, ip string, metadata map[string]interface{}) error
	Delete(handle string) (Container, error)
	ReadAll() (map[string]Container, error)
}

type Container struct {
	Handle   string                 `json:"handle"`
	IP       string                 `json:"ip"`
	Metadata map[string]interface{} `json:"metadata"`
}

type Store struct {
	Serializer serial.Serializer
	LockerNew  func(filePath string) filelock.FileLocker
}

func validate(handle, ip string) error {
	if handle == "" {
		return fmt.Errorf("invalid handle")
	}

	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid ip: %v", ip)
	}
	return nil
}

func (c *Store) Add(filePath, handle, ip string, metadata map[string]interface{}) error {
	if err := validate(handle, ip); err != nil {
		return err
	}

	locker := c.LockerNew(filePath)
	file, err := locker.Open()
	if err != nil {
		return fmt.Errorf("open lock: %s", err)
	}
	defer file.Close()

	pool := make(map[string]Container)
	err = c.Serializer.DecodeAll(file, &pool)
	if err != nil {
		return fmt.Errorf("decoding file: %s", err)
	}

	pool[handle] = Container{
		Handle:   handle,
		IP:       ip,
		Metadata: metadata,
	}

	err = c.Serializer.EncodeAndOverwrite(file, pool)
	if err != nil {
		return fmt.Errorf("encode and overwrite: %s", err)
	}

	return nil
}

func (c *Store) Delete(filePath, handle string) (Container, error) {
	deleted := Container{}
	if handle == "" {
		return deleted, fmt.Errorf("invalid handle")
	}

	locker := c.LockerNew(filePath)
	file, err := locker.Open()
	if err != nil {
		return deleted, fmt.Errorf("open lock: %s", err)
	}
	defer file.Close()

	pool := make(map[string]Container)
	err = c.Serializer.DecodeAll(file, &pool)
	if err != nil {
		return deleted, fmt.Errorf("decoding file: %s", err)
	}

	deleted = pool[handle]

	delete(pool, handle)

	err = c.Serializer.EncodeAndOverwrite(file, pool)
	if err != nil {
		return deleted, fmt.Errorf("encode and overwrite: %s", err)
	}
	return deleted, nil
}

func (c *Store) ReadAll(filePath string) (map[string]Container, error) {
	locker := c.LockerNew(filePath)
	file, err := locker.Open()
	if err != nil {
		return nil, fmt.Errorf("open lock: %s", err)
	}
	defer file.Close()

	pool := make(map[string]Container)
	err = c.Serializer.DecodeAll(file, &pool)
	if err != nil {
		return nil, fmt.Errorf("decoding file: %s", err)
	}
	return pool, nil
}
