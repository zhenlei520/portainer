package git

import (
	"context"
	"crypto/tls"
	"github.com/pkg/errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

type cloneOptions struct {
	repositoryUrl string
	username      string
	password      string
	referenceName string
	depth         int
}

type downloader interface {
	download(ctx context.Context, dst string, opt cloneOptions) error
}

type gitClient struct{
	preserveGitDirectory bool
}

func (c gitClient) download(ctx context.Context, dst string, opt cloneOptions) error {
	gitOptions := git.CloneOptions{
		URL:   opt.repositoryUrl,
		Depth: opt.depth,
	}

	if opt.password != "" || opt.username != "" {
		gitOptions.Auth = &githttp.BasicAuth{
			Username: opt.username,
			Password: opt.password,
		}
	}

	if opt.referenceName != "" {
		gitOptions.ReferenceName = plumbing.ReferenceName(opt.referenceName)
	}

	_, err := git.PlainCloneContext(ctx, dst, false, &gitOptions)

	if err != nil {
		return errors.Wrap(err, "failed to clone git repository")
	}

	if !c.preserveGitDirectory {
		os.RemoveAll(filepath.Join(dst, ".git"))
	}

	return nil
}

// Service represents a service for managing Git.
type Service struct {
	httpsCli *http.Client
	azure    downloader
	git      downloader
}

// NewService initializes a new service.
func NewService() *Service {
	httpsCli := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 300 * time.Second,
	}

	client.InstallProtocol("https", githttp.NewClient(httpsCli))

	return &Service{
		httpsCli: httpsCli,
		azure:    NewAzureDownloader(httpsCli),
		git:      gitClient{},
	}
}

// ClonePublicRepository clones a public git repository using the specified URL in the specified
// destination folder.
func (service *Service) ClonePublicRepository(repositoryURL, referenceName, destination string) error {
	return service.cloneRepository(destination, cloneOptions{
		repositoryUrl: repositoryURL,
		referenceName: referenceName,
		depth:         1,
	})
}

// ClonePrivateRepositoryWithBasicAuth clones a private git repository using the specified URL in the specified
// destination folder. It will use the specified Username and Password for basic HTTP authentication.
func (service *Service) ClonePrivateRepositoryWithBasicAuth(repositoryURL, referenceName, destination, username, password string) error {
	return service.cloneRepository(destination, cloneOptions{
		repositoryUrl: repositoryURL,
		username:      username,
		password:      password,
		referenceName: referenceName,
		depth:         1,
	})
}

func (service *Service) cloneRepository(destination string, options cloneOptions) error {
	if isAzureUrl(options.repositoryUrl) {
		return service.azure.download(context.TODO(), destination, options)
	}

	return service.git.download(context.TODO(), destination, options)
}
