# Apply MIT copyright headers to DogeGo source files (idempotent).
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT

param(
    [string]$Root = (Split-Path -Parent $PSScriptRoot)
)
$ErrorActionPreference = "Stop"

$marker = "Copyright (c) 2026 Paulo Vidal"

$goHeader = @"
// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.


"@

$psHeader = @"
# Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
# Copyright (c) 2026 Dogecoin Foundation
# SPDX-License-Identifier: MIT

"@

$blockHeader = @"
/*
 * Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
 * Copyright (c) 2026 Dogecoin Foundation
 *
 * SPDX-License-Identifier: MIT
 * See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.
 */

"@

$htmlHeader = @"
<!--
  Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
  Copyright (c) 2026 Dogecoin Foundation
  SPDX-License-Identifier: MIT
-->

"@

function Add-GoHeader([string]$filePath) {
    $content = [IO.File]::ReadAllText($filePath)
    if ($content.Contains($marker)) { return $false }
    if ($content -match '^(?<build>(?://go:build[^\r\n]*\r?\n)+)') {
        $build = $Matches['build']
        $rest = $content.Substring($build.Length)
        $new = $build + "`n" + $goHeader + $rest.TrimStart("`r", "`n")
    } else {
        $new = $goHeader + $content
    }
    [IO.File]::WriteAllText($filePath, $new)
    return $true
}

function Add-PrefixHeader([string]$filePath, [string]$header) {
    $content = [IO.File]::ReadAllText($filePath)
    if ($content.Contains($marker)) { return $false }
    [IO.File]::WriteAllText($filePath, $header + $content)
    return $true
}

$counts = @{ go = 0; ps1 = 0; js = 0; css = 0; html = 0; skip = 0; err = 0 }
$files = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object {
        $_.Name -and $_.FullName -and
        $_.FullName -notmatch '[\\/]dogedata[\\/]' -and
        $_.FullName -notmatch '[\\/]\.git[\\/]' -and
        $_.Extension -match '^\.(go|ps1|js|css|html)$'
    }

foreach ($f in $files) {
    if ($f.Name -eq 'apply_mit_license.ps1') { continue }
    try {
        $tagged = $false
        switch ($f.Extension.ToLowerInvariant()) {
            '.go' { $tagged = Add-GoHeader $f.FullName }
            '.ps1' { $tagged = Add-PrefixHeader $f.FullName $psHeader }
            '.js' { $tagged = Add-PrefixHeader $f.FullName $blockHeader }
            '.css' { $tagged = Add-PrefixHeader $f.FullName $blockHeader }
            '.html' { $tagged = Add-PrefixHeader $f.FullName $htmlHeader }
        }
        if ($tagged) { $counts[$f.Extension.TrimStart('.').ToLowerInvariant()]++ }
        else { $counts.skip++ }
    } catch {
        $counts.err++
        Write-Warning ("{0}: {1}" -f $f.FullName, $_.Exception.Message)
    }
}

Write-Host ("Tagged: go={0} ps1={1} js={2} css={3} html={4} skipped={5} errors={6}" -f `
    $counts.go, $counts.ps1, $counts.js, $counts.css, $counts.html, $counts.skip, $counts.err)
