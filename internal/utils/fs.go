package utils

import (
  "github.com/redpanda-data/benthos/v4/public/service"
  "io"
  "os"
  "path/filepath"
  "strings"
)

func LoadFileContents(filename string, fs *service.FS) ([]byte, error) {
  path, err := expandPath(filename)
  if err != nil {
    return nil, err
  }

  f, err := fs.Open(path)
  if err != nil {
    return nil, err
  }
  defer f.Close()

  return io.ReadAll(f)
}

func expandPath(p string) (string, error) {
  p = os.ExpandEnv(p)

  if !strings.HasPrefix(p, "~") {
    return p, nil
  }

  home, err := os.UserHomeDir()
  if err != nil {
    return "", err
  }

  return filepath.Join(home, p[1:]), nil
}
