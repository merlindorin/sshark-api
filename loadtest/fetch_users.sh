#!/bin/bash
# Fetch 10,000 GitHub usernames

OUTPUT_FILE="github_users.txt"
TOTAL_USERS=10000
PER_PAGE=100
SINCE=0

rm -f "$OUTPUT_FILE"
touch "$OUTPUT_FILE"

count=0
while [ $count -lt $TOTAL_USERS ]; do
    echo "Fetching users since ID $SINCE... ($count/$TOTAL_USERS)"
    
    response=$(curl -s "https://api.github.com/users?per_page=$PER_PAGE&since=$SINCE")
    
    # Check for rate limit
    if echo "$response" | jq -e '.message' > /dev/null 2>&1; then
        echo "Rate limited or error. Response:"
        echo "$response" | jq '.message'
        echo "Waiting 60s..."
        sleep 60
        continue
    fi
    
    # Extract usernames and append
    echo "$response" | jq -r '.[].login' >> "$OUTPUT_FILE"
    
    # Get last user ID for pagination
    SINCE=$(echo "$response" | jq -r '.[-1].id')
    
    count=$((count + PER_PAGE))
    
    # Small delay to avoid rate limit
    sleep 0.5
done

echo "Done! Fetched $(wc -l < "$OUTPUT_FILE") usernames to $OUTPUT_FILE"
