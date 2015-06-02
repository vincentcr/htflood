
```
 .-. .-. _______ ,---.,-.    .---.   .---.  ,'|"\
 | | | ||__   __|| .-'| |   / .-. ) / .-. ) | |\ \
 | `-' |  )| |   | `-.| |   | | |(_)| | |(_)| | \ \
 | .-. | (_) |   | .-'| |   | | | | | | | | | |  \ \
 | | |)|   | |   | |  | `--.\ `-' / \ `-' / /(|`-' /
 /(  (_)   `-'   )\|  |( __.')---'   )---' (__)`--'
(__)            (__)  (_)   (_)     (_)

```

# HTFLOOD: Distributed HTTP Load testing tool

## Request Syntax

htflood's syntax is heavily inspired from [httpie](http://httpie.org).

`
htflood -count 128 -concurrency 32 http://google.com
`

## Stats

By default htflood will output request data in json-row format. You can however pipe this output into `htflood stats`,
which will aggregate the data and output statistics:

```
htflood -count 128 -concurrency 32 http://google.com | htflood stats
```

## Distributed usage

### Start bots

#### Manually:

```
htflood bot -api-key=bigsecret -port=3210
```

#### With Docker

### Use the bots

```
htflood -bots http://localhost:3210,http://localhost:3211 -api-key=bigsecret  -count 128 -concurrency 32 http://google.com
```

When the `-bots` flag is used, htflood will split the work among the bots, instead of making the requests iself. This allows for a much larger concurrent number of requests.

