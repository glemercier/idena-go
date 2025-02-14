package database

import (
	"bytes"
	"github.com/deckarep/golang-set"
	"github.com/tendermint/tm-db"
	"sync"
)

type BackedMemDb struct {
	inner     db.DB
	permanent db.DB
	touched   mapset.Set
	mtx       sync.Mutex
}

func NewBackedMemDb(permanent db.DB) *BackedMemDb {
	return &BackedMemDb{
		inner:     db.NewMemDB(),
		permanent: permanent,
		touched:   mapset.NewSet(),
	}
}

func (db *BackedMemDb) Get(key []byte) []byte {
	if db.touched.Contains(string(key)) {
		return db.inner.Get(key)
	}
	return db.permanent.Get(key)
}

func (db *BackedMemDb) Has(key []byte) bool {
	if db.touched.Contains(string(key)) {
		return db.inner.Has(key)
	}
	return db.permanent.Has(key)
}

func (db *BackedMemDb) Set(key []byte, value []byte) {
	db.inner.Set(key, value)
	db.touch(key)
}

func (db *BackedMemDb) SetSync(key []byte, value []byte) {
	db.inner.SetSync(key, value)
	db.touch(key)
}

func (db *BackedMemDb) Delete(key []byte) {
	db.inner.Delete(key)
	db.touch(key)
}

func (db *BackedMemDb) DeleteSync(key []byte) {
	db.inner.DeleteSync(key)
	db.touch(key)
}

func (db *BackedMemDb) Iterator(start, end []byte) db.Iterator {

	innerKeys := make([][]byte, 0)
	it := db.inner.Iterator(start, end)
	for ; it.Valid(); it.Next() {
		innerKeys = append(innerKeys, it.Key())
	}
	return newIterator(db, innerKeys, db.permanent.Iterator(start, end), false)
}

func (db *BackedMemDb) ReverseIterator(start, end []byte) db.Iterator {
	innerKeys := make([][]byte, 0)
	it := db.inner.ReverseIterator(start, end)
	for ; it.Valid(); it.Next() {
		innerKeys = append(innerKeys, it.Key())
	}
	return newIterator(db, innerKeys, db.permanent.ReverseIterator(start, end), true)
}

func (db *BackedMemDb) NewBatch() db.Batch {
	return &backedMemBatch{
		batch: db.inner.NewBatch(),
		touch: db.touch,
	}
}

func (db *BackedMemDb) Close() {
	db.inner.Close()
}

func (db *BackedMemDb) Print() {
	db.inner.Print()
}

func (db *BackedMemDb) Stats() map[string]string {
	return db.inner.Stats()
}

func (db *BackedMemDb) touch(key []byte) {
	db.touched.Add(string(key))
}

type iterator struct {
	db                 *BackedMemDb
	innerKeys          [][]byte
	permanentIter      db.Iterator
	isReverse          bool
	valid              bool
	key                []byte
	value              []byte
	emptyPermanentIter bool
	empty              bool
}

func newIterator(db *BackedMemDb, innerKeys [][]byte, permanentIter db.Iterator, isReverse bool) db.Iterator {
	it := &iterator{
		db:            db,
		innerKeys:     innerKeys,
		permanentIter: permanentIter,
		isReverse:     isReverse,
		valid:         true,
	}
	it.emptyPermanentIter = !permanentIter.Valid()
	it.Next()
	return it
}

func (it *iterator) Domain() (start []byte, end []byte) {
	return it.permanentIter.Domain()
}

func (it *iterator) takeInnerKey() {
	if len(it.innerKeys) == 0 {
		it.valid = false
		it.empty = false
		return
	}

	it.key = it.innerKeys[0]
	it.value = it.db.Get(it.key)
	it.innerKeys = it.innerKeys[1:]
	it.empty = len(it.innerKeys) == 0 && it.emptyPermanentIter
}

func (it *iterator) compare(key1, key2 []byte) bool {
	if it.isReverse {
		return bytes.Compare(key1, key2) >= 0
	} else {
		return bytes.Compare(key1, key2) <= 0
	}
}

func (it *iterator) Valid() bool {
	return it.valid
}

func (it *iterator) nextPermanent() {
	if it.permanentIter.Valid() {
		it.permanentIter.Next()
	}
	it.emptyPermanentIter = !it.permanentIter.Valid()
}

func (it *iterator) Next() {
	if it.empty {
		it.valid = false
		return
	}
	for !it.emptyPermanentIter {
		if len(it.innerKeys) > 0 && it.compare(it.innerKeys[0], it.permanentIter.Key()) {
			if bytes.Compare(it.innerKeys[0], it.permanentIter.Key()) == 0 {
				it.nextPermanent()
			}
			it.takeInnerKey()
			return
		} else {

			if it.db.touched.Contains(string(it.permanentIter.Key())) {
				it.nextPermanent()
				continue
			}
			it.key = it.permanentIter.Key()
			it.value = it.permanentIter.Value()
			it.nextPermanent()
			return
		}
	}
	it.takeInnerKey()
}

func (it *iterator) Key() (key []byte) {
	return it.key
}

func (it *iterator) Value() (value []byte) {
	return it.value
}

func (it *iterator) Close() {
	it.permanentIter.Close()
}
