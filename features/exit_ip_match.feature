Feature: A rotating mobile proxy's changing exit IP is consistent, not a missing proxy
  vibe-fb's extension reports the browser's public exit IP and asks vibe-proxy
  whether it belongs to the profile's proxy. On 2026-09-15 Z-Proxy/UA/1 — a
  rotating mobile proxy on ASN 21497 (PrJSC VF UKRAINE) — showed 130 distinct
  exit IPs in 24 hours, each seen for under half an hour. The browser reported
  addresses that were never sampled, the exact lookup answered 404, and vibe-fb
  opened 12.5k "browser exit IP has no registered proxy" incidents for profiles
  that were working. For a mobile proxy a changing IP is normal: asked about
  the PINNED proxy, the same carrier network counts as consistent, and the
  answer says HOW it matched so the caller can log it.

  Background:
    Given the pinned proxy "Z-Proxy/UA/1" is a rotating mobile proxy on ASN 21497

  Scenario: the exact current exit IP matches
    Given vibe-proxy answers the check with match "exact"
    When the caller checks exit IP "46.133.7.7" against the pinned proxy
    Then the IP is consistent with match "exact"

  Scenario: an IP from the proxy's recent rotation log matches
    Given vibe-proxy answers the check with match "observed"
    When the caller checks exit IP "31.144.2.9" against the pinned proxy
    Then the IP is consistent with match "observed"

  Scenario: a never-sampled IP on the same carrier network matches
    Given vibe-proxy answers the check with match "network" and IP ASN 21497
    When the caller checks exit IP "128.124.55.1" against the pinned proxy
    Then the IP is consistent with match "network"
    And the resolved IP network is ASN 21497

  Scenario: an IP on a different network is still a mismatch
    Given vibe-proxy answers the check with mismatch "network_mismatch" and IP ASN 15895
    When the caller checks exit IP "93.72.1.1" against the pinned proxy
    Then the IP is not consistent because "network_mismatch"

  Scenario: an IP seen on another proxy is a mismatch that names that proxy
    Given vibe-proxy answers the check with mismatch "ip_observed_on_other_proxy" on proxy "Z-Proxy/UA/2"
    When the caller checks exit IP "46.133.9.9" against the pinned proxy
    Then the IP is not consistent because "ip_observed_on_other_proxy"
    And the other proxy is "Z-Proxy/UA/2"

  Scenario: a failed network lookup is transient, never a mismatch
    Given vibe-proxy cannot resolve the IP network
    When the caller checks exit IP "128.124.55.2" against the pinned proxy
    Then the check fails with reason "network_lookup_unavailable"
    And the failure does not need an operator

  Scenario: a pinned proxy that does not exist needs a human
    Given the pinned proxy does not exist
    When the caller checks exit IP "46.133.7.7" against the pinned proxy
    Then the check fails with reason "proxy_not_found"
    And the failure needs an operator
