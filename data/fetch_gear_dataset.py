"""
Fetch gear records from the CampToCamp API.

Mirrors the logic in backend/cmd/generate_gear_dataset/main.go.
"""

import time
import logging
from dataclasses import dataclass
from typing import Optional

import httpx

BASE_URL = "https://api.camptocamp.org"
DEFAULT_ACTIVITIES = ["rock_climbing", "mountain_climbing", "hiking"]
PAGE_SIZE = 50

logger = logging.getLogger(__name__)


@dataclass
class GearRecord:
    route_id: str
    gear: str

    def to_dict(self) -> dict:
        return {"route_id": self.route_id, "gear": self.gear}


def fetch_gear_dataset(
    max_records: int,
    activities: Optional[list[str]] = None,
) -> list[GearRecord]:
    """
    Fetch up to max_records gear records from CampToCamp.

    Args:
        max_records: Maximum number of records to collect across all activities.
        activities: List of C2C activity types to search. Defaults to
                    ["rock_climbing", "mountain_climbing", "hiking"].

    Returns:
        List of GearRecord objects.
    """
    if activities is None:
        activities = DEFAULT_ACTIVITIES

    client = httpx.Client(timeout=15)
    records: list[GearRecord] = []

    try:
        for activity in activities:
            if len(records) >= max_records:
                break
            logger.info("Searching activity=%s", activity)
            ids = _search_route_ids(client, activity, max_records - len(records))
            logger.info("  Found %d route IDs", len(ids))
            for route_id in ids:
                if len(records) >= max_records:
                    break
                gear = _fetch_french_gear(client, route_id)
                if not gear:
                    continue
                records.append(GearRecord(route_id=route_id, gear=gear))
                logger.info(
                    "  [%d] route %s: gear captured (%d chars)",
                    len(records),
                    route_id,
                    len(gear),
                )
                time.sleep(0.5)  # be polite to C2C API
    finally:
        client.close()

    return records


def _search_route_ids(client: httpx.Client, activity: str, max_ids: int) -> list[str]:
    ids: list[str] = []
    offset = 0
    while len(ids) < max_ids:
        url = f"{BASE_URL}/routes?act={activity}&limit={PAGE_SIZE}&offset={offset}"
        data = _api_get(client, url)
        docs = data.get("documents", [])
        if not docs:
            break
        for doc in docs:
            if isinstance(doc, dict) and "document_id" in doc:
                ids.append(str(int(doc["document_id"])))
        if len(docs) < PAGE_SIZE:
            break
        offset += PAGE_SIZE
        time.sleep(0.3)

    return ids[:max_ids]


def _fetch_french_gear(client: httpx.Client, route_id: str) -> str:
    data = _api_get(client, f"{BASE_URL}/routes/{route_id}")
    locales = data.get("locales", [])
    for lang in ("fr", "en"):
        for locale in locales:
            if isinstance(locale, dict) and locale.get("lang") == lang:
                gear = locale.get("gear", "")
                if gear:
                    return gear
    return ""


def _api_get(client: httpx.Client, url: str) -> dict:
    response = client.get(url)
    response.raise_for_status()
    return response.json()
