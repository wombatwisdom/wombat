package library

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/blevesearch/bleve/v2"
	"github.com/dgraph-io/badger/v4"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Client interface {
	Loaded() bool
	Update() error
	Search(query string, offset int, size int) (*SearchResult, error)
	Repository(repo string) (*Repository, error)
	Version(repo string, version string) (*Version, error)
	Package(repo string, version string, pkg string) (*Package, error)
	Component(repo string, version string, pkg string, name string, kind string) (*Component, error)
	Close() error
}

func New(workDir string, serverUrl string) (Client, error) {
	workDir = os.ExpandEnv(workDir)

	mapping := bleve.NewIndexMapping()
	mapping.DefaultAnalyzer = "en"
	index, err := bleve.New(path.Join(workDir, "packages.bleve"), mapping)
	if err != nil {
		return nil, err
	}

	cl := &client{
		index:     index,
		dir:       workDir,
		serverUrl: serverUrl,
	}

	if cl.Loadable() {
		if err := cl.Load(); err != nil {
			return nil, err
		}
	}

	return cl, nil
}

type client struct {
	dir       string
	kv        *badger.DB
	index     bleve.Index
	serverUrl string
}

func (s *client) Loaded() bool {
	return s.index != nil && s.kv != nil
}

func (s *client) Loadable() bool {
	_, err := os.Stat(path.Join(s.dir, "index"))
	if err != nil {
		return false
	}

	_, err = os.Stat(path.Join(s.dir, "kv"))
	if err != nil {
		return false
	}

	return true
}

func (s *client) Load() error {
	if !s.Loadable() {
		return fmt.Errorf("not loadable, run update first")
	}

	var err error
	s.kv, err = badger.Open(badger.Options{Dir: path.Join(s.dir, "kv")})
	if err != nil {
		return fmt.Errorf("failed to open kv: %w", err)
	}

	s.index, err = bleve.New(path.Join(s.dir, "index"), bleve.NewIndexMapping())
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}

	return nil
}

func (s *client) Unload() error {
	if s.index != nil {
		if err := s.index.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close index")
		}
	}

	if s.kv != nil {
		if err := s.kv.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close kv")
		}
	}

	return nil
}

func (s *client) Search(query string, offset int, size int) (*SearchResult, error) {
	if !s.Loaded() {
		return nil, fmt.Errorf("not loaded")
	}

	req := bleve.NewSearchRequest(bleve.NewQueryStringQuery(query))
	req.From = offset
	req.Size = size
	return s.search(req)
}

func (s *client) Repository(repo string) (*Repository, error) {
	key := strings.Join([]string{repo}, "/")
	var result Repository
	fnd, err := s.get(key, &result)
	if err != nil {
		return nil, err
	}
	if !fnd {
		return nil, nil
	}

	return &result, nil
}

func (s *client) Version(repo string, version string) (*Version, error) {
	key := strings.Join([]string{repo, version}, "/")
	var result Version
	fnd, err := s.get(key, &result)
	if err != nil {
		return nil, err
	}
	if !fnd {
		return nil, nil
	}

	return &result, nil
}

func (s *client) Package(repo string, version string, pkg string) (*Package, error) {
	key := strings.Join([]string{repo, version, pkg}, "/")
	var result Package
	fnd, err := s.get(key, &result)
	if err != nil {
		return nil, err
	}
	if !fnd {
		return nil, nil
	}

	return &result, nil
}

func (s *client) Component(repo string, version string, pkg string, name string, kind string) (*Component, error) {
	key := strings.Join([]string{repo, version, pkg, name, kind}, "/")
	var result Component
	fnd, err := s.get(key, &result)
	if err != nil {
		return nil, err
	}
	if !fnd {
		return nil, nil
	}

	return &result, nil
}

func (s *client) Update() error {
	// -- unload
	if err := s.Unload(); err != nil {
		return err
	}

	// -- download the file
	if err := downloadFile(s.serverUrl, s.dir); err != nil {
		return err
	}

	if !s.Loadable() {
		return fmt.Errorf("database corrupted")
	}

	return s.Load()
}

func (s *client) Close() error {
	return s.Unload()
}

func (s *client) get(key string, target any) (bool, error) {
	if !s.Loaded() {
		return false, fmt.Errorf("not loaded")
	}

	txn := s.kv.NewTransaction(false)
	item, err := txn.Get([]byte(key))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return false, nil
		}

		return false, err
	}

	codec := func(val []byte) error {
		return json.Unmarshal(val, &target)
	}

	if err := item.Value(codec); err != nil {
		return true, err
	}

	return true, nil
}

func (s *client) search(req *bleve.SearchRequest) (*SearchResult, error) {
	res, err := s.index.Search(req)
	if err != nil {
		return nil, err
	}

	result := &SearchResult{
		Total:  res.Total,
		Took:   res.Took,
		Offset: req.From,
		Size:   req.Size,
	}
	for _, hit := range res.Hits {
		var pkg IndexEntry
		if err := mapstructure.Decode(hit.Fields, &pkg); err != nil {
			log.Error().Err(err).Msg("failed to decode package")
			continue
		}

		result.Hits = append(result.Hits, pkg)
	}

	return result, nil
}

func downloadFile(url string, dir string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	archive, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}

	tr := tar.NewReader(archive)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dir, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}
