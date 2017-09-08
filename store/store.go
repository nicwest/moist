package store

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/boltdb/bolt"
	"github.com/nicwest/moist/message"
)

type Store struct {
	db *bolt.DB
}

func New(path string) (*Store, error) {
	db, err := bolt.Open("my.db", 0600, nil)
	if err != nil {
		return nil, err
	}

	store := &Store{
		db: db,
	}
	return store, err
}

func (store *Store) Save(msg *message.Message, folder string) error {

	if folder == "" {
		return fmt.Errorf("folder not specified")
	}

	err := store.db.Update(func(tx *bolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists([]byte("FOLDERS"))
		if err != nil {
			return err
		}

		fol, err := root.CreateBucketIfNotExists([]byte(folder))
		if err != nil {
			return err
		}

		msgs, err := fol.CreateBucketIfNotExists([]byte("MESSAGES"))
		if err != nil {
			return err
		}

		id, err := msgs.NextSequence()
		if err != nil {
			return err
		}

		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}

		err = msgs.Put([]byte(strconv.FormatUint(id, 10)), data)
		if err != nil {
			return err
		}

		stats, err := fol.CreateBucketIfNotExists([]byte("STATS"))
		if err != nil {
			return err
		}

		totalRaw := stats.Get([]byte("total"))
		var total, new uint64
		if len(totalRaw) > 0 {
			total, err = strconv.ParseUint(string(totalRaw), 10, 64)
			if err != nil {
				total = 0
			}
		} else {
			total = 0
		}

		total++

		err = stats.Put([]byte("total"), []byte(strconv.FormatUint(total, 10)))
		if err != nil {
			return err
		}

		newRaw := stats.Get([]byte("new"))
		if len(newRaw) > 0 {
			new, err = strconv.ParseUint(string(newRaw), 10, 64)
			if err != nil {
				new = 0
			}
		} else {
			new = 0
		}

		new++

		err = stats.Put([]byte("new"), []byte(strconv.FormatUint(total, 10)))
		if err != nil {
			return err
		}

		return nil
	})
	return err
}

func (store *Store) Folders() ([]string, error) {
	var folders []string
	err := store.db.View(func(tx *bolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists([]byte("FOLDERS"))
		if err != nil {
			return err
		}

		err = root.ForEach(
			func(k, v []byte) error {
				if v == nil {
					folders = append(folders, string(k))
				}
				return nil
			},
		)
		return nil
	})
	return folders, err
}

func (store *Store) Close() {
	store.db.Close()
}
