package git

import (
	git2go "github.com/libgit2/git2go/v31"

	"log"
)

func certificateCheckCallback(logger *log.Logger) git2go.CertificateCheckCallback {
	return func(cert *git2go.Certificate, valid bool, hostname string) git2go.ErrorCode {
		if valid {
			return git2go.ErrOk
		}

		if cert.Kind == git2go.CertificateX509 {
			if cert.X509 != nil {
				err := cert.X509.VerifyHostname(hostname)
				if err != nil {
					logger.Println("host name could not be verified")
					return git2go.ErrAuth
				}
			}
		} else if cert.Kind == git2go.CertificateHostkey {
			if cert.Hostkey.Kind == git2go.HostkeyMD5 {
				if !isByteArrayEmpty(cert.Hostkey.HashMD5[:]) {
					logger.Println("invalid host key MD5")
					return git2go.ErrAuth
				}
			} else if cert.Hostkey.Kind == git2go.HostkeySHA1 {
				if !isByteArrayEmpty(cert.Hostkey.HashSHA1[:]) {
					logger.Println("invalid host key SHA1")
					return git2go.ErrAuth
				}
			}
		}

		return git2go.ErrorCodeOK
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
