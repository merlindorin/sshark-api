#!/bin/bash
# Fetch 100,000 GitHub users (id,username)

OUTPUT_FILE="loadtest/users.csv"
TOTAL_USERS=10000000
PER_PAGE=100
SINCE=0

# Resume from existing file if present
if [ -f "$OUTPUT_FILE" ]; then
    count=$(tail -n +2 "$OUTPUT_FILE" | wc -l | tr -d ' ')
    if [ "$count" -gt 0 ]; then
        SINCE=$(tail -1 "$OUTPUT_FILE" | cut -d',' -f1)
        gum log --level info "Resuming from ID $SINCE ($count users already fetched)"
    fi
else
    echo "id,username,ingested" > "$OUTPUT_FILE"
    count=0
fi
while [ $count -lt $TOTAL_USERS ]; do
    gum log --level info "Fetching users since ID $SINCE... ($count/$TOTAL_USERS)"

    response=$(curl -s -H "Authorization: Bearer $GITHUB_API_KEY" "https://api.github.com/users?per_page=$PER_PAGE&since=$SINCE")

    # Check for rate limit
    if echo "$response" | jq -e '.message' > /dev/null 2>&1; then
        gum log --level warn "Rate limited or error. Response:"
        echo "$response" | jq '.message'
        gum log --level warn "Waiting 60s..."
        sleep 60
        continue
    fi

    # Extract id,username,ingested and append as CSV
    echo "$response" | jq -r '.[] | "\(.id),\(.login),false"' >> "$OUTPUT_FILE"

    # Get last user ID for pagination
    SINCE=$(echo "$response" | jq -r '.[-1].id')

    count=$((count + PER_PAGE))

    # Small delay to avoid rate limit
    sleep 2
done

gum log --level info "Done! Fetched $(tail -n +2 "$OUTPUT_FILE" | wc -l | tr -d ' ') users to $OUTPUT_FILE"
