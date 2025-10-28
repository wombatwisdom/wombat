package all

import (
	_ "github.com/wombatwisdom/wombat/public/components/gcp_bigtable"
	_ "github.com/wombatwisdom/wombat/public/components/mongodb/all"
	_ "github.com/wombatwisdom/wombat/public/components/nats"
	_ "github.com/wombatwisdom/wombat/public/components/redpanda"
	_ "github.com/wombatwisdom/wombat/public/components/snowflake"
	_ "github.com/wombatwisdom/wombat/public/components/splunk"
	_ "github.com/wombatwisdom/wombat/public/components/wombatwisdom/ibmmq"
	_ "github.com/wombatwisdom/wombat/public/components/wombatwisdom/mqtt3"
	_ "github.com/wombatwisdom/wombat/public/components/zeromq"
)
