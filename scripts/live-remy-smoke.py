#!/usr/bin/env python3
"""Opt-in end-to-end smoke for a running Remventory and configured OpenAI endpoint.

Run once for each configured main-model alias against a populated disposable database:
  REMVENTORY_BASE_URL=http://localhost:8080 python3 scripts/live-remy-smoke.py
"""
import http.client
import json
import os
import urllib.parse
import uuid

raw_base = os.environ.get("REMVENTORY_BASE_URL", "http://localhost:8080")
parsed_base = urllib.parse.urlsplit(raw_base)
if (
    parsed_base.scheme not in ("http", "https")
    or not parsed_base.hostname
    or parsed_base.username
    or parsed_base.password
    or parsed_base.query
    or parsed_base.fragment
):
    raise SystemExit("REMVENTORY_BASE_URL must be an HTTP(S) origin or path without credentials, query, or fragment")
base_path = parsed_base.path.rstrip("/")
token = os.environ.get("REMVENTORY_ACCESS_TOKEN", "")
if os.environ.get("REMVENTORY_SMOKE_DISPOSABLE") != "YES":
    raise SystemExit("Refusing to run without REMVENTORY_SMOKE_DISPOSABLE=YES; this scenario creates a pending proposal")
session_id = str(uuid.uuid4())
focus_ids = []


last_summary = ""
last_components = []


def api_json(path, body=None, timeout=20):
    connection_type = (
        http.client.HTTPSConnection
        if parsed_base.scheme == "https"
        else http.client.HTTPConnection
    )
    connection = connection_type(parsed_base.hostname, parsed_base.port, timeout=timeout)
    headers = {"Accept": "application/json"}
    if body is not None:
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    try:
        connection.request("POST" if body is not None else "GET", base_path + path, body=body, headers=headers)
        response = connection.getresponse()
        payload = response.read()
    finally:
        connection.close()
    if response.status < 200 or response.status >= 300:
        raise SystemExit(f"Remy request failed ({response.status}): {payload.decode(errors='replace')}")
    return json.loads(payload)


def request(message):
    global last_summary, last_components
    body = json.dumps({"message": message, "session_id": session_id, "focus_ids": focus_ids}).encode()
    result = api_json("/api/remy/request", body=body, timeout=180)
    if result.get("state") != "completed" or not result.get("summary", "").strip():
        raise SystemExit(f"Remy returned no completed natural-language answer: {result!r}")
    if result.get("session_id") != session_id:
        raise SystemExit("Remy did not preserve the requested conversation UUID")
    components = result.get("components") or []
    last_summary = result["summary"]
    last_components = components
    types = [component.get("type") for component in components]
    print(json.dumps({"summary": result["summary"], "components": types}, ensure_ascii=False))

    def walk(value):
        if isinstance(value, dict):
            for key, child in value.items():
                if key in ("id", "item_id", "category_id") and isinstance(child, str):
                    try:
                        uuid.UUID(child)
                        yield child
                    except ValueError:
                        pass
                yield from walk(child)
        elif isinstance(value, list):
            for child in value:
                yield from walk(child)

    focus_ids[:] = list(dict.fromkeys(walk(components)))[:30]
    return types


def response_has_count(response_text, components, label, expected):
    target = str(expected)
    if target in response_text:
        return True
    def visit(value):
        if isinstance(value, dict):
            if str(value.get(label, "")) == target:
                return True
            if str(value.get("label", "")).lower() == label and str(value.get("value", "")) == target:
                return True
            return any(visit(child) for child in value.values())
        if isinstance(value, list):
            return any(visit(child) for child in value)
        return False
    return visit(components)


def request_result(message):
    # request() prints a concise line; read the most recent response captured below.
    return request(message)

# Exact totals are known from scripts/smoke-seed.sql in an otherwise empty database.
count_types = request_result("How many inventory records and total units do I have? Show a statistic card.")
if "statistic" not in count_types:
    raise SystemExit("Expected a statistic presentation for inventory totals")
# Capture the visible prompt's literal numbers as well as typed component values.
# The scenario fixture has 254 records and 256 units (250 one-unit archive games plus four sets).
if not (response_has_count(last_summary, last_components, "records", 254) and response_has_count(last_summary, last_components, "units", 256)):
    raise SystemExit(f"Expected exactly 254 records and 256 units; got summary={last_summary!r}")

types = request("Show the first five items in the Smoke Games category as item cards.")
if "item_cards" not in types:
    raise SystemExit("Expected Remy to honor the item_cards presentation request")

types = request("Find the exact item Archive Game 250 and show its detail card.")
if "item_detail" not in types or "Archive Game 250" not in json.dumps(last_components):
    raise SystemExit("Expected the target after the old 200-item boundary to appear in item_detail")

types = request("Compare the first two Archive Games from my inventory.")
if "comparison" not in types:
    raise SystemExit("Expected Remy to compare canonical items retained from prior turns")

def get_json(path):
    return api_json(path)

# Proposal creation/revision are limited to a database explicitly declared disposable above.
types = request("Create a pending proposal to increase Archive Game 250 by exactly one unit. Do not approve it; show me the proposal.")
if "item_proposal" not in types:
    raise SystemExit("Expected a pending item proposal; it must not be approved by this script")
proposals = get_json("/api/proposals?limit=30").get("proposals", [])
proposal = next((p for p in proposals if p.get("status") == "pending" and "Archive Game 250" in json.dumps(p)), None)
if not proposal:
    raise SystemExit("Could not find the pending smoke proposal")
proposal_id = proposal["id"]
request("Revise that pending proposal so the quantity adjustment is plus two instead of plus one. Preserve all other fields.")
revised = get_json("/api/proposals/" + proposal_id)
payload = revised.get("proposed_payload", {})
if payload.get("quantity_delta") != 2:
    raise SystemExit(f"Expected revised pending quantity_delta=2, got {payload!r}")

print("Live Remy smoke passed; the proposal remains pending and was not approved")
