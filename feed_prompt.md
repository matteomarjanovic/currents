now let's implement the getFeed xrpc handler. it will have optional auth like searchSaves.
if the request is not authenticated, it will return a generic feed of popular saves.
if the request is authenticated, it will return a personalized feed of saves based on the viewer's interests and social graph. the degree of personalization can be controlled by the personalized parameter.

the implementation will more or less consist in two parts:
- On save arrival:
  - Insert row into saves table
  - Return 200 to client
  - Fire debounced goroutine that:
    - Loads all pin embeddings for that collection from Postgres
    - Computes medoid (O(n²) cosine distance)
    - Updates the medoid embedding on the collections row (it could be a column named canonical_embedding or something like that, i wouldn't call it medoid because we might want to experiment with other approaches later)

On getFeed call:
  - Query importance scores for all user's collections (the Postgres SUM/EXP query)
  - Sample up to 3 collections weighted by importance score (in Go)
  - Fetch precomputed medoid embedding for each sampled collection
  - Run pgvector ANN search for each medoid → get candidate pins
  - Merge + deduplicate candidates
  - Return feed

the importance score query is something like this (i've taken the formula from the PinSage paper, but we can experiment with it later):
```sql
SELECT 
    collection_id,
    SUM(EXP(-0.01 * EXTRACT(EPOCH FROM (NOW() - saved_at)) / 86400)) as importance_score
FROM "save"
WHERE user_did = $1
GROUP BY collection_id
```

for handling the personalized parameter, i was thinking to the following approach for the mvp (to keep it simple):
Interpolate between personalized and global
At the extremes:
- λ=1 (full personalization): all candidates come from pgvector ANN search on your collection medoids
- λ=0 (full exploration): candidates come from a global pool (recent/popular pins across all users)

In between, you just mix the two candidate pools proportionally:
```go
gopersonalizedCandidates := getANNCandidates(medoids, limit * alpha)
explorationCandidates  := getGlobalCandidates(limit * (1 - alpha))
feed := merge(personalizedCandidates, explorationCandidates)
```
Where alpha is the slider value. Simple, fast, works with zero extra ML.

