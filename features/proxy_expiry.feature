Feature: An expired proxy is visibly expired, not "enabled"
  On 2026-09-15, 213 proxies read status "enabled" while their term had ended.
  Every pinned lease on them answered 409 proxy_expired, and consumers could not
  tell "expired, a human must renew it" from "temporarily unavailable". Expiry
  is a date, so it never becomes a status value; vibe-proxy derives it at read
  time and says it on every proxy it returns, with the same leasability verdict
  a pinned lease would get. Nothing renews or substitutes the proxy on its own —
  it is bound to a profile.

  Scenario: an enabled proxy past its term reads as expired
    Given vibe-proxy holds proxy "proxyline/UA/1" with status "enabled", expired true and leasability "expired"
    When the caller lists proxies
    Then proxy "proxyline/UA/1" has status "enabled"
    And proxy "proxyline/UA/1" is expired with leasability "expired"
    And proxy "proxyline/UA/1" needs an operator

  Scenario: a disabled proxy past its term keeps the operator's verdict
    Given vibe-proxy holds proxy "proxyline/PL/2" with status "disabled", expired true and leasability "disabled"
    When the caller lists proxies
    Then proxy "proxyline/PL/2" is expired with leasability "disabled"

  Scenario: a proxy in term and healthy is leasable
    Given vibe-proxy holds proxy "proxyline/PL/3" with status "enabled", expired false and leasability "ok"
    When the caller lists proxies
    Then proxy "proxyline/PL/3" is not expired with leasability "ok"
    And proxy "proxyline/PL/3" does not need an operator

  Scenario: asking only for expired proxies sends the filter
    When the caller lists only expired proxies
    Then the list request carries "expired=true"

  Scenario: a pinned lease on an expired proxy needs a human, not a retry
    Given vibe-proxy rejects pinned leases with status 409 and reason "proxy_expired"
    When the caller acquires a lease pinned to the proxy
    Then the lease fails with reason "proxy_expired"
    And the failure needs an operator

  Scenario: a pinned lease on an unhealthy proxy is transient
    Given vibe-proxy rejects pinned leases with status 409 and reason "proxy_unhealthy"
    When the caller acquires a lease pinned to the proxy
    Then the lease fails with reason "proxy_unhealthy"
    And the failure does not need an operator
