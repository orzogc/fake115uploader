package cipher

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
)

var (
	gKeyL = []byte{0x42, 0xda, 0x13, 0xba, 0x78, 0x76, 0x8d, 0x37, 0xe8, 0xee, 0x04, 0x91}
	gKts  = []byte{0xf0, 0xe5, 0x69, 0xae, 0xbf, 0xdc, 0xbf, 0x5a, 0x1a, 0x45, 0xe8, 0xbe, 0x7d, 0xa6, 0x73, 0x88, 0xde, 0x8f, 0xe7, 0xc4, 0x45, 0xda, 0x86, 0x94, 0x9b, 0x69, 0x92, 0x0b, 0x6a, 0xb8, 0xf1, 0x7a, 0x38, 0x06, 0x3c, 0x95, 0x26, 0x6d, 0x2c, 0x56, 0x00, 0x70, 0x56, 0x9c, 0x36, 0x38, 0x62, 0x76, 0x2f, 0x9b, 0x5f, 0x0f, 0xf2, 0xfe, 0xfd, 0x2d, 0x70, 0x9c, 0x86, 0x44, 0x8f, 0x3d, 0x14, 0x27, 0x71, 0x93, 0x8a, 0xe4, 0x0e, 0xc1, 0x48, 0xae, 0xdc, 0x34, 0x7f, 0xcf, 0xfe, 0xb2, 0x7f, 0xf6, 0x55, 0x9a, 0x46, 0xc8, 0xeb, 0x37, 0x77, 0xa4, 0xe0, 0x6b, 0x72, 0x93, 0x7e, 0x51, 0xcb, 0xf1, 0x37, 0xef, 0xad, 0x2a, 0xde, 0xee, 0xf9, 0xc9, 0x39, 0x6b, 0x32, 0xa1, 0xba, 0x35, 0xb1, 0xb8, 0xbe, 0xda, 0x78, 0x73, 0xf8, 0x20, 0xd5, 0x27, 0x04, 0x5a, 0x6f, 0xfd, 0x5e, 0x72, 0x39, 0xcf, 0x3b, 0x9c, 0x2b, 0x57, 0x5c, 0xf9, 0x7c, 0x4b, 0x7b, 0xd2, 0x12, 0x66, 0xcc, 0x77, 0x09, 0xa6}
)

const (
	keySize   = 16
	blockSize = 128
	publicKey = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDR3rWmeYnRClwLBB0Rq0dlm8Mr
PmWpL5I23SzCFAoNpJX6Dn74dfb6y02YH15eO6XmeBHdc7ekEFJUIi+swganTokR
IVRRr/z16/3oh7ya22dcAqg191y+d6YDr4IGg/Q5587UKJMj35yQVXaeFXmLlFPo
kFiz4uPxhrB7BGqZbQIDAQAB
-----END PUBLIC KEY-----`
	privateKey = `-----BEGIN RSA PRIVATE KEY-----
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
)

// Cipher 加密解密信息
type Cipher struct {
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
	randKey    []byte
	keyS       []byte
}

// NewCipher 新建Cipher
func NewCipher() (*Cipher, error) {
	c := new(Cipher)
	block, _ := pem.Decode([]byte(publicKey))
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not a RSA public key: %+v", key)
	}
	c.publicKey = publicKey

	block, _ = pem.Decode([]byte(privateKey))
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	c.privateKey = privateKey

	return c, nil
}

// 生成key
func (c *Cipher) genKey() error {
	c.randKey = make([]byte, keySize)
	_, err := rand.Read(c.randKey)
	if err != nil {
		return err
	}
	c.keyS = genKey(c.randKey, 4)

	return nil
}

// Encrypt 加密，一次加密对应一次解密
func (c *Cipher) Encrypt(plainText []byte) ([]byte, error) {
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
	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, c.publicKey, xorText)
	if err != nil {
		return nil, err
	}
	text := make([]byte, base64.StdEncoding.EncodedLen(len(cipherText)))
	base64.StdEncoding.Encode(text, cipherText)

	return text, nil
}

// Decrypt 解密，一次加密对应一次解密
func (c *Cipher) Decrypt(cipherText []byte) ([]byte, error) {
	text := make([]byte, base64.StdEncoding.DecodedLen(len(cipherText)))
	n, err := base64.StdEncoding.Decode(text, cipherText)
	if err != nil {
		return nil, err
	}
	text = text[:n]
	blockCount := len(text) / blockSize
	plainText := make([]byte, 0, blockCount*blockSize)
	for i := 0; i < blockCount; i++ {
		t, err := rsa.DecryptPKCS1v15(rand.Reader, c.privateKey, text[i*blockSize:(i+1)*blockSize])
		if err != nil {
			return nil, err
		}
		plainText = append(plainText, t...)
	}
	randKey := plainText[:keySize]
	plainText = plainText[keySize:]
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
