package nats

import (
  "fmt"
  "github.com/nats-io/nats.go/jetstream"
  "github.com/redpanda-data/benthos/v4/public/service"
)

func parseDeliverPolicy(conf *service.ParsedConfig, key string) (jetstream.DeliverPolicy, error) {
  deliver, err := conf.FieldString(key)
  if err != nil {
    return jetstream.DeliverAllPolicy, nil
  }
  var dp jetstream.DeliverPolicy
  if err := dp.UnmarshalJSON([]byte(fmt.Sprintf("%q", deliver))); err != nil {
    return jetstream.DeliverAllPolicy, fmt.Errorf("failed to parse deliver option: %v", err)
  }
  return dp, nil
}

func parseReplayPolicy(conf *service.ParsedConfig, key string) (jetstream.ReplayPolicy, error) {
  replay, err := conf.FieldString(key)
  if err != nil {
    return jetstream.ReplayInstantPolicy, nil
  }
  var rp jetstream.ReplayPolicy
  if err := rp.UnmarshalJSON([]byte(fmt.Sprintf("%q", replay))); err != nil {
    return jetstream.ReplayInstantPolicy, fmt.Errorf("failed to parse replay option: %v", err)
  }
  return rp, nil
}

//func parseTime(conf *service.ParsedConfig, key string) (*time.Time, error) {
//  if !conf.Contains(key) {
//    return nil, nil
//  }
//  str, err := conf.FieldString(key)
//  if err != nil {
//    return nil, err
//  }
//  t, err := time.Parse(time.RFC3339, str)
//  if err != nil {
//    return nil, fmt.Errorf("failed to parse RFC3339 timestamp: %v", err)
//  }
//  return &t, nil
//}
