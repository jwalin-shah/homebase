#!/usr/bin/env python3
import json
import sys
import yaml

def compile_to_markdown(data):
    md = []
    md.append(f"# Assurance Case: {data.get('id')}")
    md.append(f"**Status:** {data.get('status').upper()}\n")
    md.append("## Claim Statement")
    md.append(f"> {data.get('statement')}\n")
    
    md.append("## Architectural Boundaries")
    for b in data.get('boundaries', []):
        md.append(f"- {b}")
    md.append("")
    
    md.append("## Hazard Disposition Ledger")
    md.append("| Hazard | Applicable | Disposition | Evidence |")
    md.append("|--------|------------|-------------|----------|")
    
    hazards = data.get('hazards', {})
    for h_name, h_data in hazards.items():
        app = "Yes" if h_data.get('applicable') else "No"
        disp = h_data.get('prevention') or h_data.get('behavior') or h_data.get('recovery') or h_data.get('residual_risk_owner') or "Unknown"
        evid = h_data.get('evidence', 'N/A')
        md.append(f"| `{h_name}` | {app} | {disp} | `{evid}` |")
    md.append("")
    
    md.append("## Environmental Assumptions")
    for i, a in enumerate(data.get('assumptions', [])):
        md.append(f"### Assumption {i+1}: {a.get('statement')}")
        enf = a.get('enforcement', {})
        md.append(f"- **Compile-time Gate:** `{enf.get('compile_time', 'none')}`")
        md.append(f"- **Test-time Gate:** `{enf.get('test_time', 'none')}`")
        md.append(f"- **Runtime Monitor:** `{enf.get('runtime', 'none')}`")
        md.append("")

    return "\n".join(md)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: compile_ast.py <ast.yaml>")
        sys.exit(1)
        
    with open(sys.argv[1], 'r') as f:
        ast_data = yaml.safe_load(f)
        
    print(compile_to_markdown(ast_data))
