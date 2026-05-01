import subprocess
import re
import os
from prompts import get_prompt_for_status, get_default_state_for_status

import subprocess
import re
import os
from prompts import get_prompt_for_status, get_default_state_for_status

def run_agent_cli(agent_type: str, prompt: str, cwd: str = None, prefix: str = "") -> str:
    """Runs the selected agent CLI with the given prompt."""
    print(f"{prefix}[*] Invoking {agent_type.capitalize()} CLI... (cwd: {cwd or 'current directory'})")
    
    policy_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "orchestrator-policy.yaml")
    
    if agent_type == "gemini":
        cmd = [
            "gemini", 
            "-p", prompt, 
            "--model", "auto",
            "--yolo",
            "--policy", policy_path
        ]
    elif agent_type == "claude":
        # Claude Code non-interactive usage
        cmd = [
            "claude", 
            "-p", prompt,
            "--bare",
            "--allowedTools", "Bash,Read,Edit"
        ]
    elif agent_type == "cursor-agent" or agent_type == "agent":
        cmd = [
            agent_type,
            "-p", prompt,
            "--yolo"
        ]
    else:
        raise ValueError(f"Unknown agent type: {agent_type}")

    process = subprocess.Popen(
        cmd,
        cwd=cwd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1
    )
    
    output = []
    
    for line in iter(process.stdout.readline, ''):
        print(f"{prefix}{line}", end='')
        output.append(line)
        
    process.stdout.close()
    return_code = process.wait()
    
    full_output = "".join(output).strip()
    
    if return_code != 0:
        print(f"{prefix}[!] {agent_type.capitalize()} CLI failed with exit code {return_code}")
        raise subprocess.CalledProcessError(return_code, cmd, output=full_output)
        
    return full_output

def parse_agent_response(output_text: str, default_state: str, prefix: str = "") -> tuple[str, str, bool]:
    """
    Parses the <NEXT_STATE> and <COMMENT> tags from the output.
    Returns a tuple of (next_state, clean_comment_text, success).
    """
    success = True
    
    # 1. Extract the next state
    state_match = re.search(r"<NEXT_STATE>(.*?)</NEXT_STATE>", output_text, re.IGNORECASE | re.DOTALL)
    if state_match:
        next_state = state_match.group(1).strip()
    else:
        print(f"{prefix}[!] Warning: Could not find <NEXT_STATE> tag in agent output.")
        next_state = default_state
        success = False

    # 2. Extract the exact comment to post (ignoring thought process)
    comment_match = re.search(r"<COMMENT>(.*?)</COMMENT>", output_text, re.IGNORECASE | re.DOTALL)
    if comment_match:
        clean_comment = comment_match.group(1).strip()
    else:
        print(f"{prefix}[!] Warning: Could not find <COMMENT> tag in agent output.")
        # Fallback: just strip the state tag and return everything else
        clean_comment = re.sub(r"<NEXT_STATE>.*?</NEXT_STATE>", "", output_text, flags=re.IGNORECASE | re.DOTALL).strip()
        success = False

    return next_state, clean_comment, success

def process_task(target_status: str, task: dict, agent_type: str = "gemini", cwd: str = None, max_retries: int = 2, prefix: str = "") -> tuple[str, str]:
    """
    Builds the prompt based on the current status, runs the selected agent, and parses the result.
    If tags are missing, it retries up to max_retries times.
    Returns (next_status, comment_body_to_post).
    """
    base_prompt = get_prompt_for_status(target_status, task)
    default_state = get_default_state_for_status(target_status)
    
    current_prompt = base_prompt
    
    for attempt in range(max_retries + 1):
        if attempt > 0:
            print(f"{prefix}[*] Retry Attempt {attempt}/{max_retries} due to missing tags...")
            
        output = run_agent_cli(agent_type, current_prompt, cwd=cwd, prefix=prefix)
        next_state, clean_comment, success = parse_agent_response(output, default_state=default_state, prefix=prefix)
        
        if success:
            return next_state, clean_comment
            
        if attempt < max_retries:
            # Append a stern correction to the prompt for the retry
            current_prompt = base_prompt + "\n\n[CRITICAL SYSTEM ERROR]: In your previous attempt, you failed to include the mandatory <COMMENT> and/or <NEXT_STATE> tags. You MUST wrap your text in <COMMENT> tags and end exactly with a <NEXT_STATE> tag as instructed above. Please try again."
            
    print(f"{prefix}[!] Error: Max retries reached. Agent failed to format output correctly. Falling back to default state.")
    return next_state, clean_comment