package storage

import (
	"fmt"
	"os"

	"github.com/jobstoit/zoomdl/config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SFTP struct {
	client *sftp.Client
}

func NewSFTP(cfg *config.SFTPConfig) (Provider, error) {
	var hostKey ssh.PublicKey
	sshConf := &ssh.ClientConfig{
		User:            cfg.User,
		HostKeyCallback: ssh.FixedHostKey(hostKey),
	}

	if cfg.Key != "" {
		key, err := os.ReadFile(cfg.Key)
		if err != nil {
			return nil, err
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}

		sshConf.Auth = append(sshConf.Auth, ssh.PublicKeys(signer))
	} else {
		sshConf.Auth = append(sshConf.Auth, ssh.Password(cfg.Password))
	}

	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), sshConf)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}

	return &SFTP{
		client: sftpClient,
	}, nil
}

// Open is an implementation of Provider
func (x SFTP) Open(path string) (File, error) {
	return x.client.Open(path)
}
