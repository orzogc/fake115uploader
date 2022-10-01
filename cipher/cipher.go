package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"hash/crc32"
	"math/big"

	"github.com/aead/ecdh"
	"github.com/andreburgaud/crypt2go/ecb"
	"github.com/andreburgaud/crypt2go/padding"
	"github.com/pierrec/lz4/v4"
)

var (
	gKeyL        = []byte{0x42, 0xda, 0x13, 0xba, 0x78, 0x76, 0x8d, 0x37, 0xe8, 0xee, 0x04, 0x91}
	gKts         = []byte{0xf0, 0xe5, 0x69, 0xae, 0xbf, 0xdc, 0xbf, 0x5a, 0x1a, 0x45, 0xe8, 0xbe, 0x7d, 0xa6, 0x73, 0x88, 0xde, 0x8f, 0xe7, 0xc4, 0x45, 0xda, 0x86, 0x94, 0x9b, 0x69, 0x92, 0x0b, 0x6a, 0xb8, 0xf1, 0x7a, 0x38, 0x06, 0x3c, 0x95, 0x26, 0x6d, 0x2c, 0x56, 0x00, 0x70, 0x56, 0x9c, 0x36, 0x38, 0x62, 0x76, 0x2f, 0x9b, 0x5f, 0x0f, 0xf2, 0xfe, 0xfd, 0x2d, 0x70, 0x9c, 0x86, 0x44, 0x8f, 0x3d, 0x14, 0x27, 0x71, 0x93, 0x8a, 0xe4, 0x0e, 0xc1, 0x48, 0xae, 0xdc, 0x34, 0x7f, 0xcf, 0xfe, 0xb2, 0x7f, 0xf6, 0x55, 0x9a, 0x46, 0xc8, 0xeb, 0x37, 0x77, 0xa4, 0xe0, 0x6b, 0x72, 0x93, 0x7e, 0x51, 0xcb, 0xf1, 0x37, 0xef, 0xad, 0x2a, 0xde, 0xee, 0xf9, 0xc9, 0x39, 0x6b, 0x32, 0xa1, 0xba, 0x35, 0xb1, 0xb8, 0xbe, 0xda, 0x78, 0x73, 0xf8, 0x20, 0xd5, 0x27, 0x04, 0x5a, 0x6f, 0xfd, 0x5e, 0x72, 0x39, 0xcf, 0x3b, 0x9c, 0x2b, 0x57, 0x5c, 0xf9, 0x7c, 0x4b, 0x7b, 0xd2, 0x12, 0x66, 0xcc, 0x77, 0x09, 0xa6}
	remotePubKey = []byte{0x57, 0xA2, 0x92, 0x57, 0xCD, 0x23, 0x20, 0xE5, 0xD6, 0xD1, 0x43, 0x32, 0x2F, 0xA4, 0xBB, 0x8A, 0x3C, 0xF9, 0xD3, 0xCC, 0x62, 0x3E, 0xF5, 0xED, 0xAC, 0x62, 0xB7, 0x67, 0x8A, 0x89, 0xC9, 0x1A, 0x83, 0xBA, 0x80, 0x0D, 0x61, 0x29, 0xF5, 0x22, 0xD0, 0x34, 0xC8, 0x95, 0xDD, 0x24, 0x65, 0x24, 0x3A, 0xDD, 0xC2, 0x50, 0x95, 0x3B, 0xEE, 0xBA}
)

const (
	rsaKeySize   = 16
	rsaBlockSize = 128
	publicRsaKey = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDR3rWmeYnRClwLBB0Rq0dlm8Mr
PmWpL5I23SzCFAoNpJX6Dn74dfb6y02YH15eO6XmeBHdc7ekEFJUIi+swganTokR
IVRRr/z16/3oh7ya22dcAqg191y+d6YDr4IGg/Q5587UKJMj35yQVXaeFXmLlFPo
kFiz4uPxhrB7BGqZbQIDAQAB
-----END PUBLIC KEY-----`
	privateRsaKey = `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQCMgUJLwWb0kYdW6feyLvqgNHmwgeYYlocst8UckQ1+waTOKHFC
TVyRSb1eCKJZWaGa08mB5lEu/asruNo/HjFcKUvRF6n7nYzo5jO0li4IfGKdxso6
FJIUtAke8rA2PLOubH7nAjd/BV7TzZP2w0IlanZVS76n8gNDe75l8tonQQIDAQAB
AoGANwTasA2Awl5GT/t4WhbZX2iNClgjgRdYwWMI1aHbVfqADZZ6m0rt55qng63/
3NsjVByAuNQ2kB8XKxzMoZCyJNvnd78YuW3Zowqs6HgDUHk6T5CmRad0fvaVYi6t
viOkxtiPIuh4QrQ7NUhsLRtbH6d9s1KLCRDKhO23pGr9vtECQQDpjKYssF+kq9iy
A9WvXRjbY9+ca27YfarD9WVzWS2rFg8MsCbvCo9ebXcmju44QhCghQFIVXuebQ7Q
pydvqF0lAkEAmgLnib1XonYOxjVJM2jqy5zEGe6vzg8aSwKCYec14iiJKmEYcP4z
DSRms43hnQsp8M2ynjnsYCjyiegg+AZ87QJANuwwmAnSNDOFfjeQpPDLy6wtBeft
5VOIORUYiovKRZWmbGFwhn6BQL+VaafrNaezqUweBRi1PYiAF2l3yLZbUQJAf/nN
4Hz/pzYmzLlWnGugP5WCtnHKkJWoKZBqO2RfOBCq+hY4sxvn3BHVbXqGcXLnZPvo
YuaK7tTXxZSoYLEzeQJBAL8Mt3AkF1Gci5HOug6jT4s4Z+qDDrUXo9BlTwSWP90v
wlHF+mkTJpKd5Wacef0vV+xumqNorvLpIXWKwxNaoHM=
-----END RSA PRIVATE KEY-----`
	p224BaseLen = 28
	crcSalt     = "^j>WD3Kr?J2gLFjD4W2y@"
)

// RsaKey 密钥
type RsaKey struct {
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
}

// NewRsaKey 新建Key
func NewRsaKey() (*RsaKey, error) {
	k := new(RsaKey)
	block, _ := pem.Decode([]byte(publicRsaKey))
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not a RSA public key: %+v", key)
	}
	k.publicKey = publicKey

	block, _ = pem.Decode([]byte(privateRsaKey))
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	k.privateKey = privateKey

	return k, nil
}

// RsaCipher 加密解密信息
type RsaCipher struct {
	key     *RsaKey
	randKey []byte
	keyS    []byte
}

// NewRsaCipher 新建Cipher
func NewRsaCipher(key *RsaKey) *RsaCipher {
	c := new(RsaCipher)
	c.key = key
	c.randKey = make([]byte, rsaKeySize)

	return c
}

// 生成key
func (c *RsaCipher) genKey() error {
	_, err := rand.Read(c.randKey)
	if err != nil {
		return err
	}
	c.keyS = genKey(c.randKey, 4)

	return nil
}

// Encrypt 加密，一次加密对应一次解密
func (c *RsaCipher) Encrypt(plainText []byte) ([]byte, error) {
	err := c.genKey()
	if err != nil {
		return nil, err
	}
	tmp := xor(plainText, c.keyS)
	for i, j := 0, len(tmp)-1; i < j; i, j = i+1, j-1 {
		tmp[i], tmp[j] = tmp[j], tmp[i]
	}
	xorText := make([]byte, 0, len(c.randKey)+len(tmp))
	xorText = append(xorText, c.randKey...)
	xorText = append(xorText, xor(tmp, gKeyL)...)
	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, c.key.publicKey, xorText)
	if err != nil {
		return nil, err
	}
	text := make([]byte, base64.StdEncoding.EncodedLen(len(cipherText)))
	base64.StdEncoding.Encode(text, cipherText)

	return text, nil
}

// Decrypt 解密，一次加密对应一次解密
func (c *RsaCipher) Decrypt(cipherText []byte) ([]byte, error) {
	text := make([]byte, base64.StdEncoding.DecodedLen(len(cipherText)))
	n, err := base64.StdEncoding.Decode(text, cipherText)
	if err != nil {
		return nil, err
	}
	text = text[:n]
	blockCount := len(text) / rsaBlockSize
	plainText := make([]byte, 0, blockCount*rsaBlockSize)
	for i := 0; i < blockCount; i++ {
		t, err := rsa.DecryptPKCS1v15(rand.Reader, c.key.privateKey, text[i*rsaBlockSize:(i+1)*rsaBlockSize])
		if err != nil {
			return nil, err
		}
		plainText = append(plainText, t...)
	}
	randKey := plainText[:rsaKeySize]
	plainText = plainText[rsaKeySize:]
	keyL := genKey(randKey, 12)
	tmp := xor(plainText, keyL)
	for i, j := 0, len(tmp)-1; i < j; i, j = i+1, j-1 {
		tmp[i], tmp[j] = tmp[j], tmp[i]
	}
	plainText = xor(tmp, c.keyS)

	return plainText, nil
}

// 生成key
func genKey(randKey []byte, keyLen int) []byte {
	xorKey := make([]byte, 0, keyLen)
	length := keyLen * (keyLen - 1)
	index := 0
	if len(randKey) != 0 {
		for i := 0; i < keyLen; i++ {
			x := byte(uint8(randKey[i]) + uint8(gKts[index]))
			xorKey = append(xorKey, gKts[length]^x)
			length -= keyLen
			index += keyLen
		}
	}

	return xorKey
}

// 利用key进行异或操作
func xor(src, key []byte) []byte {
	secret := make([]byte, 0, len(src))
	pad := len(src) % 4
	if pad > 0 {
		for i := 0; i < pad; i++ {
			secret = append(secret, src[i]^key[i])
		}
		src = src[pad:]
	}
	keyLen := len(key)
	num := 0
	for _, s := range src {
		if num >= keyLen {
			num = num % keyLen
		}
		secret = append(secret, s^key[num])
		num++
	}

	return secret
}

type EcdhCipher struct {
	key    []byte
	iv     []byte
	pubKey []byte
}

func NewEcdhCipher() (*EcdhCipher, error) {
	x := big.NewInt(0).SetBytes(remotePubKey[:p224BaseLen])
	y := big.NewInt(0).SetBytes(remotePubKey[p224BaseLen:])
	remotePublic := ecdh.Point{X: x, Y: y}

	p224 := ecdh.Generic(elliptic.P224())
	private, public, err := p224.GenerateKey(rand.Reader)

	buf := make([]byte, p224BaseLen)
	switch p := public.(type) {
	case ecdh.Point:
		p.X.FillBytes(buf)
		if big.NewInt(0).And(p.Y, big.NewInt(1)).Cmp(big.NewInt(1)) == 0 {
			buf = append([]byte{p224BaseLen + 1, 0x03}, buf...)
		} else {
			buf = append([]byte{p224BaseLen + 1, 0x02}, buf...)
		}
	default:
		return nil, fmt.Errorf("错误的public key类型")
	}
	if err != nil {
		return nil, err
	}

	secret := p224.ComputeSecret(private, remotePublic)

	cipher := new(EcdhCipher)
	cipher.key = secret[:aes.BlockSize]
	cipher.iv = secret[len(secret)-aes.BlockSize:]
	cipher.pubKey = buf
	return cipher, nil
}

func (c *EcdhCipher) Encrypt(plainText []byte) ([]byte, error) {
	pad := padding.NewPkcs7Padding(aes.BlockSize)
	data, err := pad.Pad(plainText)
	if err != nil {
		return nil, err
	}

	cipherText := make([]byte, 0, len(data))
	var xorKey []byte
	xorKey = append(xorKey, c.iv...)
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}
	mode := ecb.NewECBEncrypter(block)
	tmp := make([]byte, 0, aes.BlockSize)

	for i, b := range data {
		tmp = append(tmp, b^xorKey[i%aes.BlockSize])
		if i%aes.BlockSize == aes.BlockSize-1 {
			mode.CryptBlocks(xorKey, tmp)
			cipherText = append(cipherText, xorKey...)
			tmp = make([]byte, 0, aes.BlockSize)
		}
	}

	return cipherText, nil
}

func (c *EcdhCipher) Decrypt(cipherText []byte) (text []byte, e error) {
	defer func() {
		if err := recover(); err != nil {
			e = fmt.Errorf("%v", err)
		}
	}()

	cipherText = cipherText[0 : len(cipherText)-len(cipherText)%aes.BlockSize]

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, err
	}

	lz4Block := make([]byte, len(cipherText))
	mode := cipher.NewCBCDecrypter(block, c.iv)
	mode.CryptBlocks(lz4Block, cipherText)

	length := int(lz4Block[0]) + int(lz4Block[1])<<8
	text = make([]byte, 0x2000)
	l, err := lz4.UncompressBlock(lz4Block[2:length+2], text)
	if err != nil {
		return nil, err
	}

	return text[:l], nil
}

func (c *EcdhCipher) EncodeToken(timestamp int64) (string, error) {
	random, err := rand.Int(rand.Reader, big.NewInt(256))
	if err != nil {
		return "", err
	}
	r1 := byte(random.Uint64())
	random, err = rand.Int(rand.Reader, big.NewInt(256))
	if err != nil {
		return "", err
	}
	r2 := byte(random.Uint64())
	tmp := make([]byte, 0, 48)

	time := make([]byte, 4)
	binary.BigEndian.PutUint32(time, uint32(timestamp))

	for i := 0; i < 15; i++ {
		tmp = append(tmp, c.pubKey[i]^r1)
	}
	tmp = append(tmp, []byte{r1, 0x73 ^ r1}...)
	for i := 0; i < 3; i++ {
		tmp = append(tmp, r1)
	}
	for i := 0; i < 4; i++ {
		tmp = append(tmp, r1^time[3-i])
	}
	for i := 15; i < len(c.pubKey); i++ {
		tmp = append(tmp, c.pubKey[i]^r2)
	}
	tmp = append(tmp, []byte{r2, 0x01 ^ r2}...)
	for i := 0; i < 3; i++ {
		tmp = append(tmp, r2)
	}

	crc := make([]byte, 4)
	binary.BigEndian.PutUint32(crc, crc32.ChecksumIEEE(append([]byte(crcSalt), tmp...)))

	for i := 0; i < 4; i++ {
		tmp = append(tmp, crc[3-i])
	}

	return base64.StdEncoding.EncodeToString(tmp), nil
}
