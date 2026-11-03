# 0026: DSP negotiation proof profile

Date: 2026-11-03

Status: Accepted for proof design; implementation and runtime acceptance pending.

## Context

[ADR 0009](0009-offline-dsp-catalog-export.md) limits the exporter to the catalog
data model. [ADR 0023](0023-api-only-product-integration.md) keeps authenticated
transport, credentials and data outside the controller. [#440][spike] chooses
the profile for the independent provider/consumer proof in [#441][proof].

The existing [native import checkpoint][native] feeds an unchanged signed v4.0.0
export into EDC 0.18.1 at `3a22ff4aa90213a564309b5a05851e6d7f27f9ff`.
It observed `typed-distribution-id` incompatibility. Import and transformation
do not establish negotiation, transfer, rule enforcement or independent adoption.

## Decision

### Offer identity: provider-owned resolution

Keep the exported HTTPS/URN offer ID unchanged in the catalog and the consumer's
initial request. Choose a closed provider-owned mapping to the exact native
contract definition, asset and approved access/contract policies. Prefer the
preserved dataset ID as the native reference asset ID. Bind the exported assigner
and complete rule semantics; reject unknown, stale, ambiguous or mismatched entries.
Never infer a grant from a query URL or product owner.

EDC's [ConsumerOfferResolver][resolver] accepts the request's ID string. Its
[default resolver][default-resolver] instead dereferences native ID components.
This SPI makes the mapping a feasible design, not proven runtime compatibility.
[ContractOfferId][offer-id] generates Base64 components, but its parser accepts
three literal components when decoding fails. A parseable URN is therefore not
proof of the intended asset or policy.

The [protocol service][protocol] validates participant context, authentication,
asset selection and policy, then constructs an internal canonical native offer.
Its [initial-request flow][processors] proceeds to agreement. #441 must record
the original request ID and its native asset/policy correspondence through that
flow. Any counteroffer path must preserve the alias or refuse it. Keep every
native validation; unsupported policy rules fail rather than being dropped.

Reject rewriting the export, substituting EDC's generated catalog, copying an
arbitrary native offer ID, or choosing URN components merely to satisfy its parser.
A source-level SPI alone does not establish successful registration or negotiation.

### Distribution identity: retain the observed incompatibility

Expanded JSON-LD identity alone cannot establish end-to-end output selection.
EDC's [typed distribution transformer][distribution] retains format and service
references, but not the distribution ID. Keep `typed-distribution-id` incompatible
for this native typed path, including both distinct Harbour outputs.

Choose the existing [single-output Buoy dataset][bindings] for the first reference
transfer.
The entire exported catalog remains unchanged, including its known limitation.
Array order, multiplicity or equal format/service values cannot identify a Harbour
output. A later adapter must prove exact output binding or reject that selection.
One unambiguous transfer does not erase this limitation or complete [#126][adoption].

Reject calling the complete catalog compatible because its JSON-LD IDs survive,
or silently changing the independent oracle to accept the typed identity loss.

### Participant and access: real authentication within the current ID profile

Keep HTTPS/URN participant identities. Choose a disposable proof runtime using
OAuth2 client credentials and signed JWT access tokens through EDC's
[IdentityService extension point][identity]. Configure issuer/key trust, audience,
expiry, token type/algorithms and registered client-to-participant binding explicitly.
Use reviewed libraries and the [JWT access-token profile][jwt-profile].
The provider's actual policy engine must enforce every rule used by the selected
offer; preserved rule strings and caller-controlled identity claims are insufficient.

This needs a real adapter in #441: the pinned [OAuth2 module][oauth-client] supplies
a client, not a complete IdentityService/token verifier. [MockIdentityService][mock]
parses JSON and rejects one configured client ID without cryptographic verification.
Its rejection cannot prove unauthorized negotiation or data access is denied.
The [DCP service][dcp] uses DID-bound identity and credentials; adopting that profile
requires a separate reviewed decision rather than widening this exporter's IDs.

Prove positive negotiation/content transfer and actual endpoint denials for missing
credentials, wrong signature/issuer/audience/participant and unauthorized data access.
Keep keys, tokens, policy evaluation and data in the independent participant runtime;
the controller and exporter gain no authenticated client or data-plane role.
Reject mock IAM as security evidence, unsigned claims and fabricated HTTPS aliases
for DIDs. This chooses a proof profile, not a production identity provider or
trust grant.

### Transfer format: explicit mapping for the reference path

Choose `HttpData-PULL` for the single-output reference transfer. EDC's
[pull acceptance][http-pull] uses that type; its [transfer parser][transfer-parser]
expects `DESTTYPE-PULL/PUSH` syntax, not an arbitrary absolute format IRI.

In the proof's explicit provider bindings, map the exact exported Buoy format
`https://example.com/transfer/buoy-pull` to `HttpData-PULL`.
This fixture IRI is an opaque identifier, not a standardized format or proof of
provider/domain ownership.
Real bindings require the provider's confirmed IRI and mapping. Reject unknown
formats before transfer; do not fetch IRIs or infer mappings from media types,
query URLs or string similarity.

The Harbour format can describe that transport, but transport mapping does not
resolve its two output identities. Record native typed selection as unsupported
until exact binding is demonstrated. Reject substituting one arbitrary native
asset or changing catalog formats to make the test pass.

## Consequences

- #440 records a design. #441 still requires runtime assembly, approved bindings,
  unchanged-byte and original-ID receipts, authenticated agreement, verified reference
  content and denial checks, with pinned artifacts and bounded execution/cleanup.
- [#389][integration] and [#126][adoption] retain their complete acceptance,
  including provider confirmation of identities, rules and formats. No
  compatibility claim is broadened here.
- The exporter remains offline and default-off under `dsp-catalog-export`; its input,
  context, policy and output bounds are unchanged. No runtime or exporter code ships.
- Gate retirement, exporter removal, DID support and production trust/adoption are
  separate decisions. A review date does not activate or retire a feature.

[spike]: https://github.com/devantler-tech/khoist/issues/440
[proof]: https://github.com/devantler-tech/khoist/issues/441
[integration]: https://github.com/devantler-tech/khoist/issues/389
[adoption]: https://github.com/devantler-tech/khoist/issues/126
[bindings]: ../../tests/dataspace/edc/bindings.json
[native]: https://github.com/devantler-tech/khoist/actions/runs/38773900452/job/116556628484
[resolver]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/spi/control-plane/contract-spi/src/main/java/org/eclipse/edc/connector/controlplane/contract/spi/offer/ConsumerOfferResolver.java
[default-resolver]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/core/control-plane/control-plane-contract/src/main/java/org/eclipse/edc/connector/controlplane/contract/offer/ConsumerOfferResolverImpl.java
[offer-id]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/spi/control-plane/contract-spi/src/main/java/org/eclipse/edc/connector/controlplane/contract/spi/ContractOfferId.java
[protocol]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/core/control-plane/control-plane-aggregate-services/src/main/java/org/eclipse/edc/connector/controlplane/services/contractnegotiation/ContractNegotiationProtocolServiceImpl.java
[processors]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/core/control-plane/control-plane-contract/src/main/java/org/eclipse/edc/connector/controlplane/contract/negotiation/NegotiationProcessorsImpl.java
[distribution]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/data-protocols/dsp/dsp-lib/dsp-catalog-lib/dsp-catalog-transform-lib/src/main/java/org/eclipse/edc/protocol/dsp/catalog/transform/to/JsonObjectToDistributionTransformer.java
[identity]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/spi/common/core-spi/src/main/java/org/eclipse/edc/spi/iam/IdentityService.java
[jwt-profile]: https://www.rfc-editor.org/rfc/rfc9068.html#name-validating-jwt-access-tokens
[oauth-client]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/extensions/common/iam/oauth2/oauth2-client/src/main/java/org/eclipse/edc/iam/oauth2/client/Oauth2ClientExtension.java
[mock]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/extensions/common/iam/iam-mock/src/main/java/org/eclipse/edc/iam/mock/MockIdentityService.java
[dcp]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/extensions/common/iam/decentralized-claims/decentralized-claims-service/src/main/java/org/eclipse/edc/iam/decentralizedclaims/service/DcpIdentityService.java
[http-pull]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/system-tests/e2e-transfer-test/runner/src/test/java/org/eclipse/edc/test/e2e/TransferPullEndToEndTest.java
[transfer-parser]: https://raw.githubusercontent.com/eclipse-edc/Connector/3a22ff4aa90213a564309b5a05851e6d7f27f9ff/core/control-plane/control-plane-transfer/src/main/java/org/eclipse/edc/connector/controlplane/transfer/flow/TransferTypeParserImpl.java
