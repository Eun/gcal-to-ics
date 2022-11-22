package serve

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCrypt(t *testing.T) {
	tests := []struct {
		in           []byte
		keyToEncrypt string
		keyToDecrypt string
		wantOk       bool
	}{
		{
			in:           []byte("hello"),
			keyToEncrypt: "password",
			keyToDecrypt: "password",
			wantOk:       true,
		},
		{
			in:           []byte("world"),
			keyToEncrypt: "password",
			keyToDecrypt: "password",
			wantOk:       true,
		},
		{
			in:           nil,
			keyToEncrypt: "password",
			keyToDecrypt: "password",
			wantOk:       true,
		},
		{
			in: []byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut
labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea 
commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla
pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est
laborum.`),
			keyToEncrypt: "password",
			keyToDecrypt: "password",
			wantOk:       true,
		},
		{
			in:           []byte("Hello"),
			keyToEncrypt: "password1",
			keyToDecrypt: "password2",
			wantOk:       false,
		},
	}
	for i, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("test #%d", i), func(t *testing.T) {
			t.Parallel()
			cryptText := crypt(tt.keyToEncrypt, []byte(tt.in))
			decryptText, ok := decrypt(tt.keyToDecrypt, cryptText)
			require.Equal(t, tt.wantOk, ok)
			if ok {
				require.Equal(t, []byte(tt.in), decryptText)
			} else {
				require.NotEqual(t, []byte(tt.in), decryptText)
			}
		})
	}
}
