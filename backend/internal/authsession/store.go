package authsession

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Session struct {
	AccessToken string `json:"accessToken"`
	RememberMe  bool   `json:"rememberMe"`
}

type Store struct {
	path string
}

func NewStore(root string) *Store {
	root = strings.TrimSpace(root)
	if root == "" {
		return &Store{}
	}
	return &Store{
		path: filepath.Join(root, "config", "desktop-auth-session.json"),
	}
}

func (s *Store) Load() (Session, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return Session{}, nil
	}

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Session{}, nil
	}
	if err != nil {
		return Session{}, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, err
	}
	if !session.RememberMe {
		if err := s.clear(); err != nil {
			return Session{}, err
		}
		return Session{}, nil
	}
	if strings.TrimSpace(session.AccessToken) == "" {
		if err := s.clear(); err != nil {
			return Session{}, err
		}
		return Session{}, nil
	}
	return session, nil
}

func (s *Store) Save(session Session) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil
	}

	session.AccessToken = strings.TrimSpace(session.AccessToken)
	if !session.RememberMe || session.AccessToken == "" {
		return s.clear()
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(Session{
		AccessToken: session.AccessToken,
		RememberMe:  true,
	}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o600)
}

func (s *Store) clear() error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
