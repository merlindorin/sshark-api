#!/bin/bash
# Search users from CSV file via sshark API

CSV_FILE="loadtest/users.csv"

total=$(awk -F',' '$3 == "false"' "$CSV_FILE" | wc -l | tr -d ' ')
current=0
success=0
errors=0

gum log --level info "Starting search load test ($total users to process)"

# Process users where ingested is false
while IFS=',' read -r id user ingested; do
  # Skip header and already ingested
  [[ "$id" == "id" ]] && continue
  [[ "$ingested" != "false" ]] && continue

  ((current++))

  # Make request
  result=$(curl "https://sshark.app/api/v1/search/${user}?limit=10&offset=0" \
    --compressed \
    -H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:146.0) Gecko/20100101 Firefox/146.0' \
    -H 'Accept: */*' \
    -H 'Accept-Language: en-US,en;q=0.5' \
    -H 'Accept-Encoding: gzip, deflate, br, zstd' \
    -H 'Referer: https://sshark.app/' \
    -H 'Sec-GPC: 1' \
    -H 'Alt-Used: sshark.app' \
    -H 'Connection: keep-alive' \
    -H 'Sec-Fetch-Dest: empty' \
    -H 'Sec-Fetch-Mode: cors' \
    -H 'Sec-Fetch-Site: same-origin' \
    -H 'Priority: u=0' \
    -s -w "%{http_code} %{time_total}s" -o /dev/null)

  # Track success/error
  http_code=$(echo "$result" | awk '{print $1}')
  time_taken=$(echo "$result" | awk '{print $2}')

  if [[ "$http_code" == "200" ]]; then
    ((success++))
    gum log --level info "$current/$total $user $http_code $time_taken"
    sed -i '' "s/^${id},${user},false$/${id},${user},true/" "$CSV_FILE"
  else
    ((errors++))
    gum log --level error "$current/$total $user $http_code $time_taken"
  fi

  sleep 0.2
done < "$CSV_FILE"

# Final summary
gum log --level info "Load test complete: $current total, $success success, $errors errors"
