# Member Enrichment

Member enrichment lets Podium attach application-defined metadata to members returned by leaderboard read operations. Podium sends the members to a tenant-specific HTTP provider and merges the returned metadata into the leaderboard response.

Enrichment is optional and best-effort. If a provider is not configured or its request fails, Podium returns the leaderboard members without enriched metadata.

## Configuration

Configure a provider for each tenant that requires enrichment:

```yaml
enrichment:
  request_timeout: 500ms
  providers:
    my-game:
      endpoint: "https://profiles.example.com/v1/members/enrich"
      mode: best_effort
      headers:
        Authorization: "Bearer token"
      retry:
        max_attempts: 3
        initial_backoff: 50ms
        max_backoff: 500ms
  cache:
    ttl: 24h
    addr: ""
    password: ""
```

`endpoint` is the complete provider URL. Podium does not append a path to it. Optional headers can be used to authenticate provider requests.

`mode` controls provider failure behavior:

- `best_effort` is the default. Podium logs the provider error and returns leaderboard members without enriched metadata.
- `strict` fails the Podium request if enrichment fails.

Retry settings are optional. `max_attempts` includes the initial request and defaults to `1`. Podium retries transport errors, `429` responses, and `5xx` responses with exponential backoff. Other `4xx` responses and invalid response bodies are not retried.

## Provider request

Podium sends a `POST` request with a JSON body:

```json
{
  "tenant_id": "my-game",
  "leaderboard_id": "weekly-ranking",
  "members": [
    {
      "id": "player-1",
      "score": 1200,
      "rank": 3
    }
  ]
}
```

## Provider response

The provider returns metadata associated with each member ID:

```json
{
  "members": [
    {
      "id": "player-1",
      "metadata": {
        "display_name": "Alice",
        "avatar_url": "https://example.com/alice.png"
      }
    }
  ]
}
```

Podium ignores response entries whose IDs do not match requested members. Members omitted from the provider response are returned without additional metadata.

## Requesting enrichment

Include the tenant header on a leaderboard read:

```http
Tenant-Id: my-game
```

HTTP header names are case-insensitive. The tenant selects the configured provider and isolates cached enrichment data.
