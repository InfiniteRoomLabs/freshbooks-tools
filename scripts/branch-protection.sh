#!/usr/bin/env -S usage bash
#USAGE flag "--repo <repo>" help="owner/repo to protect" default="InfiniteRoomLabs/freshbooks-tools"

set -euo pipefail

echo "branch-protection: applying required-checks protection to $usage_repo main"

gh api \
  --method PUT \
  -H "Accept: application/vnd.github+json" \
  "repos/${usage_repo}/branches/main/protection" \
  -F "required_status_checks[strict]=true" \
  -f "required_status_checks[contexts][]=lib" \
  -f "required_status_checks[contexts][]=mcp" \
  -f "required_status_checks[contexts][]=cli" \
  -f "required_status_checks[contexts][]=repo-wide" \
  -F "enforce_admins=false" \
  -F "required_pull_request_reviews[required_approving_review_count]=0" \
  -F "restrictions=null" \
  -F "required_linear_history=true" \
  -F "allow_force_pushes=false" \
  -F "allow_deletions=false"

echo "branch-protection: done"
