// Package zjulogin provides logged-in HTTP clients for ZJU TYYS services.
//
// Typical usage:
//
//	auth, err := zjulogin.NewFromEnv()
//	if err != nil {
//		return err
//	}
//
//	tyys, err := auth.TYYS()
//	if err != nil {
//		return err
//	}
//
//	resp, err := tyys.VenueInfo(ctx, 0)
//
// NewFromEnv reads ZJU_USERNAME and ZJU_PASSWORD from .env.zju.
package zjulogin
