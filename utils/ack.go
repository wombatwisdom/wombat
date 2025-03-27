package utils

import "context"

func AckPassthrough(ctx context.Context, err error) error {
    return err
}
