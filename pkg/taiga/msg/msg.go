package msg

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"

	"github.com/kelindar/binary"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

type Protocol string

var (
	Wg    Protocol = "wg"
	Wss   Protocol = "wss"
	V2Ray Protocol = "v2ray"
)

type Version string

var (
	Version1 Version = "taiga_v1_alpha"
)

type Key [32]byte
type Nonce [12]byte // ChaCha20Poly1305 uses 12-byte nonce
type Type uint8

var (
	TypeData         Type = 1
	TypeHandshake    Type = 2
	TypeHandshakeAck Type = 3
)

// Handshake is sent by client to register its public key
type Handshake struct {
	ClientPublicKey Key
}

// HandshakeAck is sent by node to confirm registration
type HandshakeAck struct {
	Success bool
	Message string
}

// NextHop describes routing to the next node in circuit
type NextHop struct {
	PublicKey Key
	Protocol  Protocol
	Endpoint  string
}

// Msg is the decrypted message body
type Msg struct {
	Flags     uint32
	Timestamp int64
	NextHop   *NextHop // nil means this is the final destination
	Data      []byte   // IP packet data
}

// Header is the unencrypted part of message
type Header struct {
	Version      Version
	Type         Type
	EphemeralKey Key   // Sender's ephemeral public key for ECDH
	Nonce        Nonce // 12-byte nonce for ChaCha20Poly1305
}

// RawMsg is the wire format
type RawMsg struct {
	Header *Header
	Body   []byte // encrypted Msg
}

// CookedMsg is decrypted message
type CookedMsg struct {
	Header *Header
	Body   *Msg
}

type Endpoint interface {
	GetCreds() ([]byte, error)
	GetEndpoint() string
	GetType() Protocol
}

// Encoder encrypts messages for sending to a node
type Encoder struct {
	NodePublicKey Key // Public key of the target node
	Version       Version
}

// Decoder decrypts received messages
type Decoder struct {
	PrivateKey Key
	Version    Version
}

func NewEncoder(nodePublicKey Key) *Encoder {
	return &Encoder{
		NodePublicKey: nodePublicKey,
		Version:       Version1,
	}
}

func NewDecoder(privateKey Key) *Decoder {
	return &Decoder{
		PrivateKey: privateKey,
		Version:    Version1,
	}
}

// GenerateKeyPair generates a new Curve25519 key pair
func GenerateKeyPair() (privateKey, publicKey Key, err error) {
	if _, err = rand.Read(privateKey[:]); err != nil {
		return Key{}, Key{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	pub, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return Key{}, Key{}, fmt.Errorf("failed to derive public key: %w", err)
	}
	copy(publicKey[:], pub)

	return privateKey, publicKey, nil
}

// PublicKeyFromPrivate derives public key from private key
func PublicKeyFromPrivate(privateKey Key) (Key, error) {
	var publicKey Key
	pub, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return Key{}, fmt.Errorf("failed to derive public key: %w", err)
	}
	copy(publicKey[:], pub)
	return publicKey, nil
}

// EncryptMsg encrypts a message for the target node
func (e *Encoder) EncryptMsg(msg *Msg) (*RawMsg, error) {
	// Generate ephemeral key pair
	var ephemeralPrivate, ephemeralPublic Key
	if _, err := rand.Read(ephemeralPrivate[:]); err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	// Compute ephemeral public key
	pub, err := curve25519.X25519(ephemeralPrivate[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to compute ephemeral public key: %w", err)
	}
	copy(ephemeralPublic[:], pub)

	// Compute shared secret
	sharedSecret, err := curve25519.X25519(ephemeralPrivate[:], e.NodePublicKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	// Derive encryption key
	encKey := sha256.Sum256(sharedSecret)

	// Generate random nonce
	var nonce Nonce
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Create cipher
	cipher, err := chacha20poly1305.New(encKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Marshal and encrypt message
	data, err := binary.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	encryptedBody := cipher.Seal(nil, nonce[:], data, nil)

	header := &Header{
		Version:      e.Version,
		Type:         TypeData,
		EphemeralKey: ephemeralPublic,
		Nonce:        nonce,
	}

	return &RawMsg{Header: header, Body: encryptedBody}, nil
}

// DecryptBody decrypts a received message
func (d *Decoder) DecryptBody(rawMsg *RawMsg) (*CookedMsg, error) {
	// Compute shared secret
	sharedSecret, err := curve25519.X25519(d.PrivateKey[:], rawMsg.Header.EphemeralKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	// Derive encryption key
	encKey := sha256.Sum256(sharedSecret)

	// Create cipher
	cipher, err := chacha20poly1305.New(encKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Decrypt
	data, err := cipher.Open(nil, rawMsg.Header.Nonce[:], rawMsg.Body, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt body: %w", err)
	}

	// Unmarshal message
	msg := &Msg{}
	if err := binary.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &CookedMsg{Header: rawMsg.Header, Body: msg}, nil
}

// EncryptHandshake encrypts a handshake message for the node
func (e *Encoder) EncryptHandshake(hs *Handshake) (*RawMsg, error) {
	var ephemeralPrivate, ephemeralPublic Key
	if _, err := rand.Read(ephemeralPrivate[:]); err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	pub, err := curve25519.X25519(ephemeralPrivate[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to compute ephemeral public key: %w", err)
	}
	copy(ephemeralPublic[:], pub)

	sharedSecret, err := curve25519.X25519(ephemeralPrivate[:], e.NodePublicKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	encKey := sha256.Sum256(sharedSecret)

	var nonce Nonce
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	cipher, err := chacha20poly1305.New(encKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	data, err := binary.Marshal(hs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal handshake: %w", err)
	}

	encryptedBody := cipher.Seal(nil, nonce[:], data, nil)

	header := &Header{
		Version:      e.Version,
		Type:         TypeHandshake,
		EphemeralKey: ephemeralPublic,
		Nonce:        nonce,
	}

	return &RawMsg{Header: header, Body: encryptedBody}, nil
}

// DecryptHandshake decrypts a handshake message
func (d *Decoder) DecryptHandshake(rawMsg *RawMsg) (*Handshake, error) {
	sharedSecret, err := curve25519.X25519(d.PrivateKey[:], rawMsg.Header.EphemeralKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	encKey := sha256.Sum256(sharedSecret)

	cipher, err := chacha20poly1305.New(encKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	data, err := cipher.Open(nil, rawMsg.Header.Nonce[:], rawMsg.Body, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt handshake: %w", err)
	}

	hs := &Handshake{}
	if err := binary.Unmarshal(data, hs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal handshake: %w", err)
	}

	return hs, nil
}

// EncryptHandshakeAck encrypts a handshake ack for the client
func (e *Encoder) EncryptHandshakeAck(ack *HandshakeAck) (*RawMsg, error) {
	var ephemeralPrivate, ephemeralPublic Key
	if _, err := rand.Read(ephemeralPrivate[:]); err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	pub, err := curve25519.X25519(ephemeralPrivate[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to compute ephemeral public key: %w", err)
	}
	copy(ephemeralPublic[:], pub)

	sharedSecret, err := curve25519.X25519(ephemeralPrivate[:], e.NodePublicKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	encKey := sha256.Sum256(sharedSecret)

	var nonce Nonce
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	cipher, err := chacha20poly1305.New(encKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	data, err := binary.Marshal(ack)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ack: %w", err)
	}

	encryptedBody := cipher.Seal(nil, nonce[:], data, nil)

	header := &Header{
		Version:      e.Version,
		Type:         TypeHandshakeAck,
		EphemeralKey: ephemeralPublic,
		Nonce:        nonce,
	}

	return &RawMsg{Header: header, Body: encryptedBody}, nil
}

// DecryptHandshakeAck decrypts a handshake ack
func (d *Decoder) DecryptHandshakeAck(rawMsg *RawMsg) (*HandshakeAck, error) {
	sharedSecret, err := curve25519.X25519(d.PrivateKey[:], rawMsg.Header.EphemeralKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	encKey := sha256.Sum256(sharedSecret)

	cipher, err := chacha20poly1305.New(encKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	data, err := cipher.Open(nil, rawMsg.Header.Nonce[:], rawMsg.Body, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt ack: %w", err)
	}

	ack := &HandshakeAck{}
	if err := binary.Unmarshal(data, ack); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ack: %w", err)
	}

	return ack, nil
}
