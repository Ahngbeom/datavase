#!/bin/sh
# Prints the usage signals dv has, since it sends none itself: release
# downloads per tag, tap clones (each `brew install` clones the tap once),
# repository traffic and stars. Run weekly and keep the lines; the trend is
# the number.
#
# Needs gh logged in with push access to both repositories: GitHub only
# shows traffic to those who can push.
set -eu

repo=Ahngbeom/datavase
tap=Ahngbeom/homebrew-tap

echo "date	$(date +%F)"
echo "stars	$(gh api "repos/$repo" --jq .stargazers_count)"
echo "forks	$(gh api "repos/$repo" --jq .forks_count)"

echo
echo "release downloads (all assets)"
gh api "repos/$repo/releases" --paginate \
  --jq '.[] | "\(.tag_name)\t\([.assets[].download_count] | add)"'

echo
echo "last 14 days"
echo "tap clones	$(gh api "repos/$tap/traffic/clones" --jq '"\(.count) (\(.uniques) unique)"')"
echo "repo clones	$(gh api "repos/$repo/traffic/clones" --jq '"\(.count) (\(.uniques) unique)"')"
echo "repo views	$(gh api "repos/$repo/traffic/views" --jq '"\(.count) (\(.uniques) unique)"')"

echo
echo "referrers"
gh api "repos/$repo/traffic/popular/referrers" --jq '.[] | "\(.referrer)\t\(.count)"'
