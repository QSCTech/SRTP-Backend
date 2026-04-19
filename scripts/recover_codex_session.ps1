param(
    [Parameter(Mandatory = $true)]
    [string]$InputPath,

    [Parameter(Mandatory = $true)]
    [string]$OutputPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Fix-Mojibake {
    param(
        [AllowNull()]
        [string]$Text
    )

    if ([string]::IsNullOrEmpty($Text)) {
        return $Text
    }

    try {
        $gbk = [System.Text.Encoding]::GetEncoding(936)
        $utf8 = [System.Text.Encoding]::UTF8
        $commonChars = @(
            [string][char]0x7684, [string][char]0x4E86, [string][char]0x6211, [string][char]0x4F60,
            [string][char]0x8FD9, [string][char]0x90A3, [string][char]0x4EEC, [string][char]0x8981,
            [string][char]0x4E0D, [string][char]0x4F1A, [string][char]0x53EF, [string][char]0x4EE5,
            [string][char]0x73B0, [string][char]0x5728, [string][char]0x5982, [string][char]0x679C,
            [string][char]0x8FD8, [string][char]0x662F, [string][char]0x5904, [string][char]0x7406,
            [string][char]0x95EE, [string][char]0x9898, [string][char]0x4EE3, [string][char]0x7801,
            [string][char]0x6587, [string][char]0x4EF6, [string][char]0x4ED3, [string][char]0x5E93
        )

        $candidates = New-Object System.Collections.Generic.List[string]
        $candidates.Add($Text)

        $current = $Text
        for ($i = 0; $i -lt 3; $i++) {
            $current = $utf8.GetString($gbk.GetBytes($current))
            $candidates.Add($current)
        }

        $best = $Text
        $bestScore = 0
        foreach ($candidate in $candidates) {
            $han = ([regex]::Matches($candidate, "[\u4e00-\u9fff]")).Count
            $commonHits = 0
            foreach ($commonChar in $commonChars) {
                $commonHits += ([regex]::Matches($candidate, [regex]::Escape($commonChar))).Count
            }
            $candidateScore = ($commonHits * 20) + $han
            if ($candidateScore -gt $bestScore) {
                $best = $candidate
                $bestScore = $candidateScore
            }
        }

        return $best
    } catch {
    }

    return $Text
}

function Unescape-JsonString {
    param(
        [AllowNull()]
        [string]$Text
    )

    if ($null -eq $Text) {
        return ""
    }

    try {
        $wrapped = '"' + ($Text -replace '\\', '\\' -replace '"', '\"') + '"'
        return [System.Text.Json.JsonSerializer]::Deserialize($wrapped, [string])
    } catch {
        $t = $Text
        $t = $t -replace '\\r\\n', "`r`n"
        $t = $t -replace '\\n', "`n"
        $t = $t -replace '\\r', "`r"
        $t = $t -replace '\\"', '"'
        $t = $t -replace '\\\\', '\'
        return $t
    }
}

if (-not (Test-Path -LiteralPath $InputPath)) {
    throw "Input file not found: $InputPath"
}

$entries = New-Object System.Collections.Generic.List[object]

Get-Content -LiteralPath $InputPath | ForEach-Object {
    $line = $_

    $messageMatch = [regex]::Match(
        $line,
        '"timestamp":"(?<ts>[^"]+)".*"type":"message","role":"(?<role>user|assistant)".*"text":"(?<text>.*)"\}\],"phase"',
        [System.Text.RegularExpressions.RegexOptions]::Singleline
    )

    if ($messageMatch.Success) {
        $text = Unescape-JsonString $messageMatch.Groups["text"].Value
        $text = Fix-Mojibake $text
        $entries.Add([pscustomobject]@{
            Timestamp = $messageMatch.Groups["ts"].Value
            Role = $messageMatch.Groups["role"].Value
            Text = $text
        })
        return
    }

    $eventUserMatch = [regex]::Match(
        $line,
        '"timestamp":"(?<ts>[^"]+)".*"type":"user_message","message":"(?<text>.*)","images":\[\],"local_images":\[\],"text_elements":\[\]',
        [System.Text.RegularExpressions.RegexOptions]::Singleline
    )

    if ($eventUserMatch.Success) {
        $text = Unescape-JsonString $eventUserMatch.Groups["text"].Value
        $text = Fix-Mojibake $text
        $entries.Add([pscustomobject]@{
            Timestamp = $eventUserMatch.Groups["ts"].Value
            Role = "user_event"
            Text = $text
        })
        return
    }

    $agentMatch = [regex]::Match(
        $line,
        '"timestamp":"(?<ts>[^"]+)".*"type":"agent_message","message":"(?<text>.*)","phase":"(?<phase>[^"]*)"',
        [System.Text.RegularExpressions.RegexOptions]::Singleline
    )

    if ($agentMatch.Success) {
        $text = Unescape-JsonString $agentMatch.Groups["text"].Value
        $text = Fix-Mojibake $text
        $entries.Add([pscustomobject]@{
            Timestamp = $agentMatch.Groups["ts"].Value
            Role = "assistant_event"
            Text = $text
        })
    }
}

$entries = $entries | Sort-Object Timestamp, Role, Text -Unique

$lines = New-Object System.Collections.Generic.List[string]
$lines.Add("# Recovered Codex Session")
$lines.Add("")
$lines.Add("- Source: ``$InputPath``")
$lines.Add("- Recovered at: $(Get-Date -Format "yyyy-MM-dd HH:mm:ss zzz")")
$lines.Add("- Entries extracted: $($entries.Count)")
$lines.Add("- Note: this transcript was rebuilt from a damaged JSONL session file, so a few messages may be duplicated or partially truncated.")
$lines.Add("")

foreach ($entry in $entries) {
    $roleLabel = switch ($entry.Role) {
        "user" { "User" }
        "assistant" { "Assistant" }
        "user_event" { "User Event" }
        "assistant_event" { "Assistant Event" }
        default { $entry.Role }
    }

    $lines.Add("## [$($entry.Timestamp)] $roleLabel")
    $lines.Add("")
    foreach ($textLine in ($entry.Text -split "`r?`n")) {
        $lines.Add($textLine)
    }
    $lines.Add("")
}

$outputDir = Split-Path -Parent $OutputPath
if ($outputDir -and -not (Test-Path -LiteralPath $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllLines($OutputPath, $lines, $utf8NoBom)

Write-Output "Recovered entries: $($entries.Count)"
Write-Output "Output: $OutputPath"
