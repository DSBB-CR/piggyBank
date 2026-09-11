package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

type Transaction struct {
	ID     string    `json:"id"`
	Amount float64   `json:"amount"`
	Note   string    `json:"note"`
	Date   time.Time `json:"date"`
}

type Account struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Currency     string        `json:"currency"`
	Initial      float64       `json:"initial"`
	Transactions []Transaction `json:"transactions"`
	Created      time.Time     `json:"created"`
}

func (a Account) Balance() float64 {
	b := a.Initial
	for _, t := range a.Transactions {
		b += t.Amount
	}
	return b
}

func (a Account) Income() float64 {
	var s float64
	for _, t := range a.Transactions {
		if t.Amount > 0 {
			s += t.Amount
		}
	}
	return s
}

func (a Account) Expense() float64 {
	var s float64
	for _, t := range a.Transactions {
		if t.Amount < 0 {
			s += t.Amount
		}
	}
	return s
}

// SortedTransactions — свежие сверху
func (a Account) SortedTransactions() []Transaction {
	txs := make([]Transaction, len(a.Transactions))
	copy(txs, a.Transactions)
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].Date.After(txs[j].Date)
	})
	return txs
}

type Store struct {
	mu       sync.RWMutex
	path     string
	accounts map[string]*Account
}

func NewStore(path string) *Store {
	return &Store{path: path, accounts: map[string]*Account{}}
}

func (s *Store) Load() error {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var list []*Account
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	for _, a := range list {
		s.accounts[a.ID] = a
	}
	return nil
}

func (s *Store) save() error {
	list := make([]*Account, 0, len(s.accounts))
	for _, a := range s.accounts {
		list = append(list, a)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Created.Before(list[j].Created) })
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0644)
}

func (s *Store) Accounts() []*Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Account, 0, len(s.accounts))
	for _, a := range s.accounts {
		list = append(list, a)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Created.Before(list[j].Created) })
	return list
}

func (s *Store) Account(id string) (*Account, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.accounts[id]
	return a, ok
}

func (s *Store) CreateAccount(name, currency string, initial float64) *Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := &Account{
		ID:       newID(),
		Name:     name,
		Currency: currency,
		Initial:  initial,
		Created:  time.Now(),
	}
	s.accounts[a.ID] = a
	s.save()
	return a
}

func (s *Store) DeleteAccount(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[id]; !ok {
		return false
	}
	delete(s.accounts, id)
	s.save()
	return true
}

func (s *Store) AddTransaction(accID string, amount float64, note string) (Transaction, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[accID]
	if !ok {
		return Transaction{}, false
	}
	tx := Transaction{
		ID:     newID(),
		Amount: amount,
		Note:   note,
		Date:   time.Now(),
	}
	a.Transactions = append(a.Transactions, tx)
	s.save()
	return tx, true
}

func (s *Store) DeleteTransaction(accID, txID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[accID]
	if !ok {
		return false
	}
	for i, t := range a.Transactions {
		if t.ID == txID {
			a.Transactions = append(a.Transactions[:i], a.Transactions[i+1:]...)
			s.save()
			return true
		}
	}
	return false
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}