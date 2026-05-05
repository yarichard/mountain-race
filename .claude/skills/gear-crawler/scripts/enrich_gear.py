#!/usr/bin/env python3
import json
from dotenv import load_dotenv
import sys
import os
from openai import OpenAI

load_dotenv()

SYSTEM_PROMPT = """You are a mountain climbing equipment assistant. Parse the following gear description and return a JSON array. Each element must have exactly three fields:
\t- "name": equipment name (string, in french)
\t- "quantity": number needed (integer, 1 if unspecified)
\t- "notes": "optional" or "mandatory" (translated in french), plus any relevant detail (string, in french)
The name of these equipments are related with the mountain activities. You should only point out personal equipment, for instance quickdraws or rope. 
When equipment refers to a specific size of friends, you should keep it as a separated element. 
For instance: "camalots #0.75, #1, #2, #3 et éventuellement #4" should remain as 5 different elements with quantity 1, not merged in a single 'camalots' with a quantity of 5.
You should include only equipment you're absolutely sure about. Output ONLY the JSON array, no explanation."""

def parse_equipment(client, gear_text):
    response = client.chat.completions.create(
        model="gpt-4.1-nano-2025-04-14",
        messages=[
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": gear_text},
        ],
        temperature=0,
    )
    content = response.choices[0].message.content.strip()
    return json.loads(content)

def main():
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        print("OPENAI_API_KEY not set", file=sys.stderr)
        sys.exit(1)

    client = OpenAI(api_key=api_key)
    input_path = sys.argv[1] if len(sys.argv) > 1 else "./test.jsonl"
    output_path = input_path + ".tmp"

    with open(input_path) as fin, open(output_path, "w") as fout:
        for i, line in enumerate(fin, 1):
            record = json.loads(line)
            gear = record.get("gear", "")
            print(f"[{i}] Processing route {record['route_id']}...", end=" ", flush=True)
            try:
                equipment = parse_equipment(client, gear)
                record["equipment"] = equipment
                print(f"{len(equipment)} items")
            except Exception as e:
                print(f"ERROR: {e}")
                record["equipment"] = []
            fout.write(json.dumps(record, ensure_ascii=False) + "\n")

    os.replace(output_path, input_path)
    print("Done.")

if __name__ == "__main__":
    main()
