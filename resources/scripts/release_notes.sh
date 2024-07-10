#!/bin/sh
echo "For installation instructions check out the [getting started guide](https://www.benthos.dev/docs/guides/getting_started)."
cat CHANGELOG.md | awk '
  /^## [0-9]/ {
      release++;
  }
  /TBD$/ {
      print "";
      print "NOTE: This is a release candidate, you can download a binary from this page or pull a docker image from https://github.com/wombatwisdom/wombat/pkgs/container/wombat with the specific tag of the release candidate.";
  }
  !/^## [0-9]/ {
      if ( release == 1 ) print;
      if ( release > 1 ) exit;
  }'
echo "The full change log can be [found here](https://github.com/wombatwisdom/wombat/blob/main/CHANGELOG.md)."
