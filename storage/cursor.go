package storage

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
)

type cursor struct {
	Current string `json:"current"`
}

func (c *cursor) String() string {
	b, err := json.MarshalIndent(c, " ", " ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func loadCursor(ctx context.Context, storageClient *storage.Client, bucketName, rootStoragePath string) (*cursor, error) {
	log.Println("Load cursor file")

	rootedCursorStoragePath := joinObjectPaths(rootStoragePath, cursorStoragePath)
	objectReader, err := storageClient.Bucket(bucketName).Object(rootedCursorStoragePath).NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		log.Printf("Cursor file doesn't exist, most likely a first run (path: %s)", rootedCursorStoragePath)
		return new(cursor), nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "can't read the cursor file (path: %s)", rootedCursorStoragePath)
	}
	defer objectReader.Close()

	b, err := ioutil.ReadAll(objectReader)
	if err != nil {
		return nil, errors.Wrapf(err, "ioutil.ReadAll failed")
	}

	var c cursor
	err = json.Unmarshal(b, &c)
	if err != nil {
		return nil, errors.Wrapf(err, "can't unmarshal the cursor file")
	}

	log.Printf("Loaded cursor file: %s", c.String())
	return &c, nil
}
