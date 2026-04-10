"""Solve TYYS aj-captcha clickWord challenges.

Input:
  - stdin, or a JSON file path argument
  - accepts either the whole /api/captcha/get response or its repData/data object

Output:
  - stdout JSON with fields suitable for /api/captcha/check:
    captchaType, token, pointJson, click_points
"""

from __future__ import annotations

import argparse
import base64
import json
import sys
import time
from collections import Counter
from io import BytesIO
from typing import Any

import ddddocr
import numpy as np
from PIL import Image, ImageDraw, ImageOps
from scipy.optimize import linear_sum_assignment


def encrypt_aj_captcha_text(text: str, secret_key: str | None, mode: str) -> str:
    if mode == "plain":
        return text
    if not secret_key:
        return text
    try:
        from Crypto.Cipher import AES
        from Crypto.Util.Padding import pad
    except ImportError as exc:
        raise RuntimeError("pycryptodome is required when CAPTCHA response contains secretKey") from exc

    key = secret_key.encode("utf-8")
    if mode == "cbc":
        cipher = AES.new(key, AES.MODE_CBC, iv=key)
    else:
        cipher = AES.new(key, AES.MODE_ECB)
    encrypted = cipher.encrypt(pad(text.encode("utf-8"), AES.block_size))
    return base64.b64encode(encrypted).decode("ascii")


def preprocess_variants(crop_img: Image.Image, size: int = 64) -> list[bytes]:
    results: list[bytes] = []
    rotations: list[int] = [0, -30, -20, -10, 10, 20, 30, -45, 45, -60, 60]

    for angle in rotations:
        rotated: Image.Image = crop_img.rotate(angle, expand=True, fillcolor=(255, 255, 255))
        resized: Image.Image = rotated.resize((size, size))

        buf = BytesIO()
        resized.save(buf, format="PNG")
        results.append(buf.getvalue())

        gray: Image.Image = ImageOps.autocontrast(ImageOps.grayscale(resized), cutoff=10)
        buf = BytesIO()
        gray.save(buf, format="PNG")
        results.append(buf.getvalue())

        arr: np.ndarray = np.array(gray)
        for thresh in [110, 140]:
            binary: Image.Image = Image.fromarray(((arr < thresh) * 255).astype(np.uint8))
            buf = BytesIO()
            binary.save(buf, format="PNG")
            results.append(buf.getvalue())

            inverted: Image.Image = Image.fromarray(((arr >= thresh) * 255).astype(np.uint8))
            buf = BytesIO()
            inverted.save(buf, format="PNG")
            results.append(buf.getvalue())

    return results


def extract_rep_data(payload: dict[str, Any]) -> dict[str, Any]:
    if "repData" in payload:
        return payload["repData"]
    if "data" in payload and isinstance(payload["data"], dict):
        data = payload["data"]
        if "repData" in data:
            return data["repData"]
        if "originalImageBase64" in data:
            return data
    if "originalImageBase64" in payload:
        return payload
    raise ValueError("captcha response does not contain repData/originalImageBase64")


def parse_points_json(points_json: str | None) -> list[dict[str, int]] | None:
    if not points_json:
        return None
    parsed = json.loads(points_json)
    if not isinstance(parsed, list):
        raise ValueError("--points-json must be a JSON array")

    points: list[dict[str, int]] = []
    for item in parsed:
        if not isinstance(item, dict) or "x" not in item or "y" not in item:
            raise ValueError("--points-json items must contain x and y")
        points.append({"x": int(item["x"]), "y": int(item["y"])})
    return points


def solve(
    payload: dict[str, Any],
    annotate_path: str | None = None,
    aes_mode: str = "ecb",
    manual_points_json: str | None = None,
) -> dict[str, Any]:
    rep: dict[str, Any] = extract_rep_data(payload)
    word_list: list[str] = rep["wordList"]
    img_bytes: bytes = base64.b64decode(rep["originalImageBase64"])
    img: Image.Image = Image.open(BytesIO(img_bytes))
    manual_points = parse_points_json(manual_points_json)

    print(f"Image: {img.size}, words={word_list}", file=sys.stderr)
    detector = ddddocr.DdddOcr(det=True, show_ad=False)
    ocr = ddddocr.DdddOcr(show_ad=False)

    t0 = time.time()
    boxes: list[list[int]] = detector.detection(img_bytes)
    print(f"Detection: {len(boxes)} boxes in {time.time() - t0:.3f}s", file=sys.stderr)

    draw = ImageDraw.Draw(img)
    crops: list[dict[str, Any]] = []

    for i, box in enumerate(boxes):
        x1, y1, x2, y2 = box
        cx = (x1 + x2) / 2
        cy = (y1 + y2) / 2
        cropped = img.crop((x1, y1, x2, y2))
        variants = preprocess_variants(cropped)

        char_counter: Counter[str] = Counter()
        for variant in variants:
            try:
                text = ocr.classification(variant).strip()
                for ch in text:
                    char_counter[ch] += 1
            except Exception:
                pass

        print(f"crop#{i} ({cx:.0f},{cy:.0f}) -> {char_counter.most_common(5)}", file=sys.stderr)
        crops.append({"cx": cx, "cy": cy, "box": box, "counts": char_counter})
        draw.rectangle([x1, y1, x2, y2], outline="#bbbbbb", width=1)

    if not crops:
        raise ValueError("captcha detector returned no boxes")

    if manual_points is not None:
        points = manual_points
        print(f"Using manual click points: {json.dumps(points, ensure_ascii=False)}", file=sys.stderr)
        for point_index, point in enumerate(points):
            x = point["x"]
            y = point["y"]
            draw.ellipse([x - 7, y - 7, x + 7, y + 7], outline="lime", width=3)
            draw.text((x + 8, max(0, y - 14)), f"{point_index + 1}", fill="lime")
    else:
        score_matrix = np.zeros((len(word_list), len(crops)))
        for word_index, word in enumerate(word_list):
            for crop_index, crop in enumerate(crops):
                score_matrix[word_index, crop_index] = crop["counts"].get(word, 0)

        row_ind, col_ind = linear_sum_assignment(-score_matrix)
        click_points: list[dict[str, int] | None] = [None] * len(word_list)

        for word_index, crop_index in zip(row_ind, col_ind):
            crop = crops[crop_index]
            score = score_matrix[word_index, crop_index]
            word = word_list[word_index]
            print(
                f"'{word}' -> crop#{crop_index} ({crop['cx']:.0f},{crop['cy']:.0f}) hits={score:.0f}",
                file=sys.stderr,
            )
            click_points[word_index] = {"x": round(crop["cx"]), "y": round(crop["cy"])}
            x1, y1, x2, y2 = crop["box"]
            draw.rectangle([x1, y1, x2, y2], outline="lime", width=3)
            draw.text((x1, max(0, y1 - 14)), f"{word_index + 1}:{word}", fill="lime")

        if any(point is None for point in click_points):
            raise ValueError("captcha assignment did not cover every word")

        points: list[dict[str, int]] = [point for point in click_points if point is not None]
    print(f"Ordered click points: {json.dumps(points, ensure_ascii=False)}", file=sys.stderr)
    if annotate_path:
        img.save(annotate_path)

    token = rep.get("token", "")
    point_json_plain = json.dumps(points, separators=(",", ":"), ensure_ascii=False)
    secret_key = rep.get("secretKey")
    point_json = encrypt_aj_captcha_text(point_json_plain, secret_key, aes_mode)
    captcha_verification = encrypt_aj_captcha_text(f"{token}---{point_json_plain}", secret_key, aes_mode) if token else ""
    print(
        f"Captcha crypto: mode={aes_mode} secretKeyLen={len(secret_key or '')} plainPointJson={point_json_plain}",
        file=sys.stderr,
    )

    captcha_type = rep.get("captchaType") or "clickWord"
    output = {
        "captchaType": captcha_type,
        "token": token,
        "pointJson": point_json,
        "captchaVerification": captcha_verification,
        "values": {
            "captchaType": captcha_type,
            "token": token,
            "pointJson": point_json,
        },
        "click_points": points,
    }
    print(f"Total time: {time.time() - t0:.3f}s", file=sys.stderr)
    return output


def load_input(path: str | None) -> dict[str, Any]:
    if path:
        with open(path, "r", encoding="utf-8") as file:
            return json.load(file)
    return json.loads(sys.stdin.buffer.read().decode("utf-8"))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("input", nargs="?")
    parser.add_argument("--annotate", default=None)
    parser.add_argument("--aes-mode", choices=["ecb", "cbc", "plain"], default="ecb")
    parser.add_argument("--points-json", default=None)
    args = parser.parse_args()

    result = solve(load_input(args.input), args.annotate, args.aes_mode, args.points_json)
    print(json.dumps(result, ensure_ascii=False, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
