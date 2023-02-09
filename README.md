# trusted_proxy module for caddy

This module retrieves cloudflare ips from their offical website, [ipv4](https://www.cloudflare.com/ips-v4) and [ipv6](https://www.cloudflare.com/ips-v6). It is supported from caddy v2.6.3 onwards.

# Example config

Put following config in global options under corresponding server options

```
trusted_proxies cloudflare {
    interval 12h
    timeout 15s
}
```

# Defaults

| Name     | Description                                            | Type     | Default    |
|----------|--------------------------------------------------------|----------|------------|
| interval | How often cloudflare ip lists are retrieved            | duration | 1h         |
| timeout  | Maximum time to wait to get a response from cloudflare | duration | no timeout |