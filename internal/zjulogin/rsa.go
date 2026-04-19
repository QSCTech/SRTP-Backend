package zjulogin

import (
	"fmt"
	"math/big"
)

func rsaEncryptPassword(password string, exponentHex string, modulusHex string) (string, error) {
	pwd := big.NewInt(0)
	for _, c := range password {
		pwd.Mul(pwd, big.NewInt(256))
		pwd.Add(pwd, big.NewInt(int64(c)))
	}

	n := new(big.Int)
	if _, ok := n.SetString(modulusHex, 16); !ok {
		return "", fmt.Errorf("parse rsa modulus")
	}
	e := new(big.Int)
	if _, ok := e.SetString(exponentHex, 16); !ok {
		return "", fmt.Errorf("parse rsa exponent")
	}

	crypt := new(big.Int).Exp(pwd, e, n)
	return fmt.Sprintf("%0*s", len(modulusHex), crypt.Text(16)), nil
}
