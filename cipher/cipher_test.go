package cipher

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/valyala/fastjson"
)

func TestParseKey(t *testing.T) {
	block, _ := pem.Decode([]byte(publicRsaKey))
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Errorf("public key parsing error: %v", err)
	}
	if _, ok := key.(*rsa.PublicKey); !ok {
		t.Errorf("public key is not a RSA public key: %+v", key)
	}

	block, _ = pem.Decode([]byte(privateRsaKey))
	_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Errorf("private key parsing error: %v", err)
	}
}

func TestGenKey(t *testing.T) {
	randKey := []byte{50, 106, 98, 84, 84, 110, 119, 89, 87, 75, 84, 71, 53, 110, 121, 120}
	key := genKey(randKey, 4)
	keyS := []byte{95, 51, 195, 33}
	if !bytes.Equal(key, keyS) {
		t.Errorf("keyS want: %v, result: %v", keyS, key)
	}

	randKey = []byte{122, 53, 116, 69, 109, 102, 111, 48, 98, 68, 114, 100, 107, 105, 55, 101}
	key = genKey(randKey, 12)
	keyL := []byte{54, 182, 181, 92, 119, 41, 196, 52, 191, 101, 11, 48}
	if !bytes.Equal(key, keyL) {
		t.Errorf("keyL want: %v, result: %v", keyL, key)
	}
}

func TestXor(t *testing.T) {
	src := []byte{123, 36, 112, 100, 77, 107, 44, 111, 55, 101, 75, 58, 46, 98, 80, 50, 20, 117, 33, 122, 134, 108, 125, 92, 48, 125, 56, 54, 120, 128, 34, 185}
	key := []byte{23, 24, 239, 61}
	secret := xor(src, key)
	want := []byte{108, 60, 159, 89, 90, 115, 195, 82, 32, 125, 164, 7, 57, 122, 191, 15, 3, 109, 206, 71, 145, 116, 146, 97, 39, 101, 215, 11, 111, 152, 205, 132}
	if !bytes.Equal(secret, want) {
		t.Errorf("xor text want: %v, result: %v", want, secret)
	}

	src = []byte{64, 215, 94, 111, 5, 115, 92, 35, 97, 189, 216, 155, 76, 133, 189, 138, 15, 144, 123, 59, 71, 205, 135, 115, 85, 170, 115, 146, 84, 169, 51, 118}
	key = []byte{66, 218, 19, 186, 120, 118, 141, 55, 232, 238, 4, 145}
	secret = xor(src, key)
	want = []byte{2, 13, 77, 213, 125, 5, 209, 20, 137, 83, 220, 10, 14, 95, 174, 48, 119, 230, 246, 12, 175, 35, 131, 226, 23, 112, 96, 40, 44, 223, 190, 65}
	if !bytes.Equal(secret, want) {
		t.Errorf("xor text want: %v, result: %v", want, secret)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	t.Skip("Need pickcode and cookie")
	const downURL = "http://proapi.115.com/app/chrome/downurl"
	key, err := NewRsaKey()
	if err != nil {
		t.Errorf("create key error: %v", err)
	}
	c := NewRsaCipher(key)
	text, err := c.Encrypt([]byte(fmt.Sprintf(`{"pickcode":"%s"}`, "needPickcode")))
	if err != nil {
		t.Errorf("encrypt error: %v", err)
	}
	t.Log(string(text))
	client := &http.Client{}
	form := url.Values{}
	form.Set("data", string(text))
	req, err := http.NewRequest(http.MethodPost, downURL, strings.NewReader(form.Encode()))
	if err != nil {
		t.Errorf("create request error: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 115disk/26.2.2")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "yourCookie")
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("http post request error: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("%+v", resp.Header)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("read response body error: %v", err)
	}
	t.Log(string(body))
	var p fastjson.Parser
	v, err := p.ParseBytes(body)
	if err != nil {
		t.Errorf("json parse error: %v", err)
	}
	if !v.GetBool("state") {
		t.Error("encrypt failed")
	}
	text, err = c.Decrypt(v.GetStringBytes("data"))
	if err != nil {
		t.Errorf("decrypt error: %v", err)
	}
	t.Log(string(text))
}
