## Go Bookshelf App on GAE

This is based on the sample that comes with the SDK documentation plus a few extra things:

+ Cache the landing page (uses Memcache)
+ Search for books (uses Search API)
 
#### Dev/Deploy

```
$> make build
...
$> make deploy-worker
...
$> make deploy-app
...
```