"""
Authentication utilities for DistWorker client
"""

import hashlib
import hmac
from datetime import datetime
from typing import Dict

AUTH_PREFIX = "DISTWORKER1"
SIGNING_KEY_DATA = "distworker1_request"
WS_SIGNING_KEY_DATA = "distworker1_websocket"
DATE_FORMAT = "%Y%m%dT%H%M%SZ"
DATE_ONLY_FORMAT = "%Y%m%d"

class ValidateContext:
    def __init__(self):
        self.worker_id: str = ""
        self.signed_headers: str = ""
        self.provided_signature: str = ""
        self.date: datetime = datetime.now()
        self.headers: Dict[str, str] = {}
        self.hashed_payload: str = ""
        self.method: str = ""
        self.url: str = ""

def _hmac_sha256(key: bytes, data: bytes) -> bytes:
    """HMAC-SHA256 해시 생성"""
    return hmac.new(key, data, hashlib.sha256).digest()

def canonical_request(method: str, uri: str, query_string: str,
                      headers: Dict[str, str], signed_headers: list[str],
                      hashed_payload: str) -> str:
    """정규화된 요청 문자열 생성"""
    canonical_headers_parts = []

    # 헤더 정렬
    sorted_headers = sorted(signed_headers)

    for header in sorted_headers:
        header_lower = header.lower()
        if header_lower in headers:
            value = headers[header_lower].strip()
            canonical_headers_parts.append(f"{header_lower}:{value}")

    canonical_headers_str = "\n".join(canonical_headers_parts) + "\n"
    signed_headers_str = ";".join(sorted_headers)

    return f"{method.upper()}\n{uri}\n{query_string}\n{canonical_headers_str}{signed_headers_str}\n{hashed_payload}"

def generate_signature(worker_token: str, date: str, canonical_request: str) -> str:
    """HMAC-SHA256 서명 생성"""
    # DateKey = HMAC-SHA256(key = "DISTWORKER1" + worker_token, date = "<YYYYMMDD>")
    date_key = _hmac_sha256((AUTH_PREFIX + worker_token).encode(), date.encode())

    # SigningKey = HMAC-SHA256(key = <DateKey>, data = "distworker1_request")
    signing_key = _hmac_sha256(date_key, SIGNING_KEY_DATA.encode())

    # Signature = HEX(HMAC-SHA256(key = <SigningKey>, data = <CanonicalRequest>))
    signature = _hmac_sha256(signing_key, canonical_request.encode())

    return signature.hex()

def generate_websocket_signature(worker_token: str, date: str, data: bytes) -> bytes:
    """WebSocket용 HMAC-SHA256 서명 생성"""
    date_key = _hmac_sha256((AUTH_PREFIX + worker_token).encode(), date.encode())

    signing_key = _hmac_sha256(date_key, WS_SIGNING_KEY_DATA.encode())

    return _hmac_sha256(signing_key, data)

def new_validate_context(
        headers: Dict[str, str],
        method: str = "GET",
        url: str = ""
) -> ValidateContext:
    """HTTP 요청에서 ValidateContext 생성"""
    vctx = ValidateContext()
    vctx.method = method
    vctx.url = url

    # Authorization 헤더 추출
    auth_header = headers.get("authorization", "")
    if not auth_header.startswith("DISTWORKER1_HMAC_SHA256"):
        raise ValueError("invalid authorization header format")

    # Authorization 헤더 파싱
    auth_content = auth_header[len("DISTWORKER1_HMAC_SHA256 "):]
    auth_parts = auth_content.split(", ")
    auth_map = {}

    for part in auth_parts:
        if "=" in part:
            key, value = part.split("=", 1)
            auth_map[key] = value

    # 필요한 필드 추출
    vctx.worker_id = auth_map.get("WorkerId", "")
    vctx.signed_headers = auth_map.get("SignedHeaders", "")
    vctx.provided_signature = auth_map.get("Signature", "")

    # 날짜 파싱
    date_str = auth_map.get("Date", "")
    if date_str:
        vctx.date = datetime.strptime(date_str, DATE_FORMAT)

    # 헤더 정규화 (소문자로 변환)
    vctx.headers = {k.lower(): v for k, v in headers.items()}

    return vctx

def validate_timestamp(timestamp: datetime, tolerance_seconds: int = 300) -> bool:
    """타임스탬프 유효성 검증"""
    now = datetime.utcnow()
    diff = abs((now - timestamp).total_seconds())
    return diff <= tolerance_seconds

def build_http_authorization_header(worker_id: str, worker_token: str,
                                         signed_headers: list[str]) -> str:
    """Http 용 인증 헤더 생성"""
    now = datetime.utcnow()
    date_str = now.strftime(DATE_FORMAT)

    # 서명된 헤더 문자열 생성
    signed_headers_str = ";".join(sorted(signed_headers))

    # 정규화된 요청 생성을 위한 기본값들
    canonical_req = canonical_request("GET", "/", "", {}, signed_headers, "")

    # 서명 생성
    date_only = now.strftime(DATE_ONLY_FORMAT)
    signature = generate_signature(worker_token, date_only, canonical_req)

    return (f"DISTWORKER1_HMAC_SHA256 WorkerId={worker_id}, "
            f"Date={date_str}, "
            f"SignedHeaders={signed_headers_str}, "
            f"Signature={signature}")
