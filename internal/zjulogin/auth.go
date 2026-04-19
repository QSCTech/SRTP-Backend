package zjulogin

import "sync"

type Auth struct {
	am     *ZJUAM
	config Config

	mu   sync.Mutex
	tyys *TYYS
}

func (a *Auth) ZJUAM() *ZJUAM {
	return a.am
}

func (a *Auth) TYYS() (*TYYS, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.tyys == nil {
		service, err := NewTYYS(a.am, a.config.TYYSSignSecret)
		if err != nil {
			return nil, err
		}
		a.tyys = service
	}
	return a.tyys, nil
}
