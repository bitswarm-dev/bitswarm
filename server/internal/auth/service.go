package auth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"bitswarm/server/internal/docstore"
)

const (
	kvKey       = "auth"
	maxFails    = 5
	lockFor     = 30 * time.Second
	demoUser    = "admin"
	demoPass    = "swarm"
	pbkdfRounds = 120000
	SessionTTL  = 30 * 24 * time.Hour
)

var ErrShortPassword = errors.New("Use at least 6 characters for the Web UI password.")

type UserSource interface {
	Username() string
	LocalBypass() bool
}

type Logger interface {
	Add(level, src, msg string)
}

type stored struct {
	Salt string `json:"salt"`
	Hash string `json:"hash"`
}

type session struct {
	User    string
	Expires time.Time
}

type Result struct {
	OK        bool
	Token     string
	TriesLeft int
	LockedFor time.Duration
	Message   string
}

type Service struct {
	kv    *docstore.KV
	users UserSource
	log   Logger
	now   func() time.Time

	mu       sync.Mutex
	cred     stored
	custom   bool
	sessions map[string]session
	fails    int
	lockedAt time.Time
}

func NewService(kv *docstore.KV, users UserSource, log Logger) (*Service, error) {
	s := &Service{kv: kv, users: users, log: log, now: time.Now, sessions: map[string]session{}}
	ok, err := kv.Load(context.Background(), kvKey, &s.cred)
	if err != nil {
		return nil, err
	}
	s.custom = ok
	return s, nil
}

func derive(pw, salt string) (string, error) {
	key, err := pbkdf2.Key(sha256.New, pw, []byte(salt), pbkdfRounds, 32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Service) UsingDemoPassword() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.custom
}

func (s *Service) SetPassword(ctx context.Context, pw string) error {
	if len(pw) < 6 {
		return ErrShortPassword
	}
	salt := randomHex(16)
	h, err := derive(pw, salt)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cred, s.custom = stored{Salt: salt, Hash: h}, true
	s.mu.Unlock()
	s.log.Add("info", "Sign-in", "The Web UI password was changed")
	return s.kv.Save(ctx, kvKey, s.cred)
}

func (s *Service) ResetPassword(ctx context.Context) error {
	s.mu.Lock()
	s.cred, s.custom = stored{}, false
	s.mu.Unlock()
	return s.kv.Delete(ctx, kvKey)
}

func (s *Service) check(pw string) bool {
	if !s.custom {
		return subtle.ConstantTimeCompare([]byte(pw), []byte(demoPass)) == 1
	}
	h, err := derive(pw, s.cred.Salt)
	return err == nil && subtle.ConstantTimeCompare([]byte(h), []byte(s.cred.Hash)) == 1
}

func (s *Service) Login(user, pw string) Result {
	s.mu.Lock()
	defer s.mu.Unlock()

	if left := lockFor - s.now().Sub(s.lockedAt); !s.lockedAt.IsZero() && left > 0 {
		return Result{LockedFor: left, Message: fmt.Sprintf("Too many tries. Try again in %d seconds.", int(left.Seconds())+1)}
	}
	if s.fails >= maxFails {
		s.fails = 0
	}

	if user == s.users.Username() && s.check(pw) {
		s.fails, s.lockedAt = 0, time.Time{}
		token := randomHex(32)
		s.sessions[token] = session{User: user, Expires: s.now().Add(SessionTTL)}
		s.log.Add("info", "Sign-in", fmt.Sprintf("%s signed in", user))
		return Result{OK: true, Token: token}
	}

	s.fails++
	if s.fails >= maxFails {
		s.lockedAt = s.now()
		s.log.Add("warn", "Sign-in", fmt.Sprintf("Sign-in locked for 30 seconds after %d failed tries", maxFails))
		return Result{LockedFor: lockFor, Message: "Too many tries. Try again in 30 seconds."}
	}
	left := maxFails - s.fails
	s.log.Add("warn", "Sign-in", fmt.Sprintf("Failed sign-in for “%s”", user))
	return Result{TriesLeft: left, Message: fmt.Sprintf("Wrong username or password. %s left.", tries(left))}
}

func tries(n int) string {
	if n == 1 {
		return "1 try"
	}
	return fmt.Sprintf("%d tries", n)
}

func (s *Service) Valid(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok {
		return false
	}
	if s.now().After(sess.Expires) {
		delete(s.sessions, token)
		return false
	}
	return true
}

func (s *Service) Logout(token string) {
	s.mu.Lock()
	sess, ok := s.sessions[token]
	delete(s.sessions, token)
	s.mu.Unlock()
	if ok {
		s.log.Add("info", "Sign-in", sess.User+" signed out")
	}
}

func (s *Service) LocalBypass() bool { return s.users.LocalBypass() }

func (s *Service) Username() string { return s.users.Username() }
