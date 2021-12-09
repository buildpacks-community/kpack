package git

import (
	git2go "github.com/libgit2/git2go/v33"
	"github.com/pkg/errors"
)

func certificateCheckCallback() git2go.CertificateCheckCallback {
	return func(cert *git2go.Certificate, valid bool, hostname string) error {
		if valid {
			return nil
		}

		if cert.Kind == git2go.CertificateX509 {
			if cert.X509 != nil {
				err := cert.X509.VerifyHostname(hostname)
				if err != nil {
					return errors.Wrap(err, "host name could not be verified")
				}
			}
		} else if cert.Kind == git2go.CertificateHostkey {
			if cert.Hostkey.Kind == git2go.HostkeyMD5 {
				if !isByteArrayEmpty(cert.Hostkey.HashMD5[:]) {
					return errors.New("invalid host key MD5")
				}
			} else if cert.Hostkey.Kind == git2go.HostkeySHA1 {
				if !isByteArrayEmpty(cert.Hostkey.HashSHA1[:]) {
					return errors.New("invalid host key SHA1")
				}
			}
		}

		return nil
	}

}

func isByteArrayEmpty(byteArray []byte) bool {
	isEmpty := true
	for _, v := range byteArray {
		if v != 0 {
			isEmpty = false
		}
	}
	return isEmpty
}
