// gate_inproc_table.go —— `zerg gate check` 的**进程内（inproc）分发表**（缺口 `GAP-20261001-75`）。
//
// 病（缺口逐字）：inproc 的**分布表**与**放行名单**混在 `family_gate_check.go` 里 ⇒
// 语义两件事挤一处（分布表 = 哪些判据已真搬成 Go、可在进程内跑；放行名单 = 命令面准入）。
// 治法（已拍 · 方案 A）：把分布表**归位**到本件（一件只管一件事），`family_gate_check.go` 只留
// 壳逻辑与放行面（引用本件的 `gateCheckGoEntry`，名字一字未改 ⇒ 零随动）。
//
// ★ 本件是纯**归位**：表内容逐字未动（只搬家、不改一行判据）。
package main

import "io"

var gateCheckGoEntry = map[string]func(stdout, stderr io.Writer, argv []string) int{
	"affected-steps":                        affectedStepsGateMain,
	"atomic-write":                          atomicWriteGateMain,
	"audit_go_logs":                         auditGoLogsGateMain,
	"baseline-ratchet":                      baselineRatchetGateMain,
	"bench-summary":                         benchSummaryGateMain,
	"check-affected-map":                    checkAffectedMapGateMain,
	"check-affected-reprove":                checkAffectedReproveGateMain,
	"check-ai-direct-edits":                 aidirecteditsGateMain,
	"check-anchor-uniqueness":               anchorUniquenessGateMain,
	"check-approve-backfill":                checkApproveBackfillGateMain,
	"check-approve-ci":                      checkApproveCIGateMain,
	"check-approve-closedset":               checkApproveClosedsetGateMain,
	"check-approve-freeze":                  checkApproveFreezeGateMain,
	"check-approve-merge":                   checkApproveMergeGateMain,
	"check-approve-tier":                    checkApproveTierGateMain,
	"check-approve-time":                    checkApproveTimeGateMain,
	"check-archive-mount":                   checkArchiveMountGateMain,
	"check-artifact-source-parity":          checkArtifactSourceParityGateMain,
	"check-build-selfclean":                 buildSelfcleanGateMain,
	"check-build-tags":                      checkBuildTagsGateMain,
	"check-bypass-idioms":                   checkBypassIdiomsGateMain,
	"check-ci-package-parity":               ciPackageParityGateMain,
	"check-cli-contract":                    cliContractGateMain,
	"check-compat-manifest":                 compatManifestGateMain,
	"check-contract-registry":               contractRegistryGateMain,
	"check-contract-version-diff":           cvdMain,
	"check-curl-timeout":                    checkCurlTimeoutGateMain,
	"check-danger-guard-parity":             dangerGuardParityMain,
	"check-dangling-refs":                   checkDanglingRefsGateMain,
	"check-delegation-selfreport":           checkDelegationSelfreportGateMain,
	"check-dev-targets":                     checkDevTargetsGateMain,
	"check-doc-cmds":                        checkDocCmdsGateMain,
	"check-doc-consumers":                   checkDocConsumersGateMain,
	"check-doc-freshness":                   checkDocFreshnessGateMain,
	"check-doc-meta":                        checkDocMetaGateMain,
	"check-doc-name":                        checkDocNameGateMain,
	"check-doc-section-numbers":             checkDocSectionNumbersGateMain,
	"check-docs-orphans-links":              checkDocsOrphansLinksGateMain,
	"check-dryrun-semantics":                checkDryrunSemanticsGateMain,
	"check-dryrun-yes-conflict-coverage":    checkDryrunYesConflictCoverageGateMain,
	"check-empty-domain-guards":             checkEmptyDomainGuardsGateMain,
	"check-env-knobs":                       checkEnvKnobsGateMain,
	"check-error-kinds":                     errorKindsGateMain,
	"check-exec-two-condition":              checkExecTwoConditionGateMain,
	"check-exit-codes":                      checkExitCodesGateMain,
	"check-face-parity":                     checkFaceParityGateMain,
	"check-fields-face-parity":              checkFieldsFaceParityGateMain,
	"check-flag-registry-parity":            flagRegistryParityGateMain,
	"check-fleet-loadable":                  checkFleetLoadableGateMain,
	"check-gap-axis-reconcile":              checkGapAxisReconcileGateMain,
	"check-gap-closure-drift":               checkGapClosureDriftGateMain,
	"check-gap-ledger-face":                 checkGapLedgerFaceGateMain,
	"check-gap-ledger-view":                 gapLedgerViewGateMain,
	"check-gap-state-vs-artifact":           checkGapStateVsArtifactGateMain,
	"check-gate-coverage":                   checkGateCoverageGateMain,
	"check-gate-point-output":               checkGatePointOutputGateMain,
	"check-gate-script-registered":          checkGateScriptRegisteredGateMain,
	"check-genflag-surface":                 checkGenflagSurfaceGateMain,
	"check-ghost-decl-diff":                 checkGhostDeclDiffGateMain,
	"check-git-env-hygiene":                 gitEnvHygieneGateMain,
	"check-git-path-outlet":                 gitPathOutletGateMain,
	"check-glossary":                        checkGlossaryGateMain,
	"check-gotoolchain":                     gotoolchainGateMain,
	"check-hardcoded-private-paths":         hardcodedPrivatePathsGateMain,
	"check-help-shape":                      checkHelpShapeGateMain,
	"check-help-snapshot":                   checkHelpSnapshotGateMain,
	"check-history-secrets":                 checkHistorySecretsGateMain,
	"check-i18n":                            checkI18nGateMain,
	"check-i18n-drift":                      checkI18nDriftGateMain,
	"check-idspace-map":                     checkIdspaceMapGateMain,
	"check-judge-ledger":                    judgeLedgerGateMain,
	"check-judge-registry":                  checkJudgeRegistryGateMain,
	"check-keep-marks":                      keepMarksGateMain,
	"check-lang-freeze":                     checkLangFreezeGateMain,
	"check-lineref-limit":                   checkLinerefLimitGateMain,
	"check-links-lychee":                    checkLinksLycheeGateMain,
	"check-loose-exit-registry":             checkLooseExitRegistryGateMain,
	"check-mainrepo-md":                     checkMainrepoMdGateMain,
	"check-manifest-freshness":              mfMain,
	"check-manifest-freshness-profile":      checkManifestFreshnessProfileGateMain,
	"check-manifest-freshness-two-criteria": checkManifestFreshnessTwoCriteriaGateMain,
	"check-marks-coverage-parity":           checkMarksCoverageParityGateMain,
	"check-mkdir-temp-cleanup":              checkMkdirTempCleanupGateMain,
	"check-model-size-anchor":               checkModelSizeAnchorGateMain,
	"check-multirepo-summary":               checkMultirepoSummaryGateMain,
	"check-mutation-residue":                mutationResidueGateMain,
	"check-newcmd-followup":                 checkNewcmdFollowupGateMain,
	"check-newfile-registry":                checkNewfileRegistryGateMain,
	"check-no-symlink-farm":                 checkNoSymlinkFarmGateMain,
	"check-nodes-roster":                    checkNodesRosterGateMain,
	"check-outrepo-dead-refs":               checkOutrepoDeadRefsGateMain,
	"check-partial-run-acceptance":          checkPartialRunAcceptanceGateMain,
	"check-parser-discipline":               parserDisciplineGateMain,
	"check-password-argv":                   checkPasswordArgvGateMain,
	"check-pending-decisions":               checkPendingDecisionsGateMain,
	"check-placeholder-residue":             checkPlaceholderResidueGateMain,
	"check-private-path-forms":              privatePathFormsGateMain,
	"check-public-ci-green":                 cicgGateMain,
	"check-public-face-anchor":              checkPublicFaceAnchorGateMain,
	"check-public-face-commands":            checkPublicFaceCommandsGateMain,
	"check-public-face-pairing":             checkPublicFacePairingGateMain,
	"check-public-face-wording":             checkPublicFaceWordingGateMain,
	"check-public-marks":                    checkPublicMarksGateMain,
	"check-public-marks-deep":               checkPublicMarksDeepGateMain,
	"check-public-marks-tracked-parity":     checkPublicMarksTrackedParityGateMain,
	"check-public-tree-hazards":             checkPublicTreeHazardsGateMain,
	"check-public-tree-private":             checkPublicTreePrivateGateMain,
	"check-public-tree-selfcheck":           checkPublicTreeSelfcheckGateMain,
	"check-publish-face-sync":               checkPublishFaceSyncGateMain,
	"check-publish-layer-names":             checkPublishLayerNamesGateMain,
	"check-py-go-pairing-drift":             pgpdGateMain,
	"check-record-not-evidence":             checkRecordNotEvidenceGateMain,
	"check-registry-key-unique":             checkRegistryKeyUniqueGateMain,
	"check-registry-vs-disk":                registryVsDiskGateMain,
	"check-replace-rule-fixtures":           checkReplaceRuleFixturesGateMain,
	"check-repo-symlink-farm":               checkRepoSymlinkFarmGateMain,
	"check-retire-consistency":              checkRetireConsistencyGateMain,
	"check-retire-roster":                   retireRosterGateMain,
	"check-selfcount":                       checkSelfcountGateMain,
	"check-selftest-knobs":                  checkSelftestKnobsGateMain,
	"check-service-declaration":             checkServiceDeclarationGateMain,
	"check-shell-unicode-vars":              checkShellUnicodeVarsGateMain,
	"check-signature-identity":              checkSignatureIdentityGateMain,
	"check-slice":                           checkSliceGateMain,
	"check-stale-frozen":                    csfGateMain,
	"check-step-switch":                     stepSwitchGateMain,
	"check-stepcount-drift":                 stepcountDriftGateMain,
	"check-symbol-delete-ripple":            symbolDeleteRippleGateMain,
	"check-token-argv":                      checkTokenArgvGateMain,
	"check-tool-version-sync":               checkToolVersionSyncGateMain,
	"check-trustroot-witness":               checkTrustrootWitnessGateMain,
	"check-tty-output-contract":             checkTtyOutputContractGateMain,
	"check-two-pass-identity":               checkTwoPassIdentityGateMain,
	"check-unsubstituted-placeholders":      unsubstitutedPlaceholdersGateMain,
	"check-verdict-record":                  checkVerdictRecordGateMain,
	"check-verify-cmd-real":                 checkVerifyCmdRealGateMain,
	"check-version-sources":                 checkVersionSourcesGateMain,
	"check-wall-layer":                      walllayerGateMain,
	"check-wallclock-ratchet":               checkWallclockRatchetMain,
	"check-wired-scripts":                   checkWiredScriptsGateMain,
	"check-write-authz":                     writeAuthzGateMain,
	"check-write-face-gate":                 checkWriteFaceGateGateMain,
	"check-writer-guard":                    checkWriterGuardGateMain,
	"check-zero-side-effect":                checkZeroSideEffectGateMain,
	"check-zh-en":                           checkZhEnGateMain,
	"check_docs":                            checkDocsGateMain,
	"check_version":                         versionGateMain,
	"compare":                               evalCompareGateMain,
	"decl-label":                            declLabelGateMain,
	"deploy-launch":                         deployLaunchGateMain,
	"doc-index-gen":                         docIndexGenGateMain,
	"doc-site-build":                        docSiteBuildGateMain,
	"fleet-logs":                            fleetLogsGateMain,
	"gate-check-health":                     gateCheckHealthMain,
	"gate-report":                           gateReportMain,
	"gen-cli-reference":                     genCliReferenceGateMain,
	"gen-gap-ledger":                        genGapLedgerGateMain,
	"gen-glossary-exports":                  genGlossaryExportsGateMain,
	"gen-tools-md":                          genToolsMdGateMain,
	"i18n-audit":                            i18nAuditGateMain,
	"inplace-window":                        ipwGateMain,
	"isolate-ro":                            isolateRoGateMain,
	"input-fingerprint":                     inputFingerprintGateMain,
	"judge-lock":                            judgeLockGateMain,
	"kb-docs-sync":                          kbDocsSyncGateMain,
	"m2-go-py-isomorphism-scan":             m2GoPyIsomorphismScanGateMain,
	"pack-release":                          packReleaseMain,
	"preview-gate-steps":                    pgsGateMain,
	"repo-diffstat-check":                   repoDiffstatGateMain,
	"seal-ledger":                           sealLedgerGateMain,
	"show-baseline":                         showBaselineGateMain,
	"stability-stats":                       evalStabilityStatsGateMain,
	"two-states-gate-mutate":                twoStatesMutateGateMain,
	"ui-start":                              uiStartGateMain,
	"who-references":                        whoReferencesGateMain,
	"writer-lock":                           writerLockGateMain,
	"writeout-probe":                        writeoutProbeGateMain,
	"zerg-egg":                              zergEggGateMain,
}

// ── 中央分类登记（写工具危险档的真源 · GAP-20261008-11）────────────────────────────
//
// 病（缺口逐字）：写工具「默认档要授权」这道壳侧闸，此前靠**现读件头** `#zerg-class: write D2|D3`
// （`family_gate_check.go` 的 gateCheckClassOf）⇒ **件一离盘**（退役 = 件头随件消失），分类读空 ⇒
// 那道闸**静默失明**（`--dry-run` 不再出计划件、裸跑直落子件）。实测：`scripts/dev/atomic-write.sh`
// 在位 ⇒ `zerg gate check atomic-write --dry-run` rc=0；把同一件移出仓 ⇒ rc=2（档位判定随件蒸发）。
//
// 治法：分类**上移**到本中央表（与 `gateCheckGoEntry` 同处 · Go 常量），按**判据名**登记 ——
// 与件在不在盘**无关** ⇒ 退役前后判定**逐字同**。件头 `#zerg-class:` 降级为**派生副本**（门禁侧
// `check-writer-guard.py` 的 G0/G1/G2 仍按件头做登记面机检）。
//
// ★ 新增写工具 ⇒ **同批**在此登记（✗ 不许只在件头写：那样它一退场，这道闸就消失）。

// gateCheckClassSpec —— 判据名的中央分类一格（`class` ＋ 危险档 `level`）。
type gateCheckClassSpec struct {
	Class string // "write"（写工具/危险档）| "read"（只读 · 不受闸）
	Level string // write 档必需：dangerD2 / dangerD3；read 档取空
}

// gateCheckClassEntry —— 【分类真源】判据名 ⇒ 危险档（✗ 不再靠读件头 · 见上「病」）。
var gateCheckClassEntry = map[string]gateCheckClassSpec{
	"atomic-write": {"write", dangerD2}, // scripts/dev/atomic-write.sh（换件正门）
	"isolate-ro":   {"write", dangerD2}, // scripts/dev/isolate-ro.sh
	// ★ A 方案（2026-10-08）· **重编类/写档类**上移登记（判据名 ⇒ write D2 · 与件在不在盘无关）。
	//   病根：名录按目录派生 ⇒ 这些件一退场，甲目录判（gateCheckRebuildDirRel）读空、乙行为判
	//   （gateCheckWritesRepoArtifact）读不到件 ⇒ 若未登本表则「D2 确认盾」静默消失（实测
	//   scripts/build/pack-release.sh 移出仓后裸跑 rc 2→1，无门会红）。登入本表 ⇒ 退役前后判定逐字同。
	//   实际只读件（zerg-swap-agent/zerg-swap-ui/test-publish-parity/
	//   publish-snapshot-self-test）✗ 不登记 ⇒ 行为对它们与今日逐字同（退役即失盾 · 本不需盾）。
	"build-all":         {"write", dangerD2}, // scripts/build/build-all.sh（真重编 + 写 repo bin/）
	"build-wall":        {"write", dangerD2}, // scripts/build/build-wall.sh（cargo release + 写 repo bin/zerg-wall）
	"pack-release":      {"write", dangerD2}, // scripts/build/pack-release.sh（真重编 + 写发布目录 $REL）
	"publish-preflight": {"write", dangerD2}, // scripts/build/publish-preflight.sh（发布树重编 + 写 /tmp）
	"publish-public":    {"write", dangerD2}, // scripts/build/publish-public.sh（真写公开快照 $OUT）
	"run_all":           {"write", dangerD2}, // scripts/build/run_all.sh（真写仓外 eval 存档；2026-10-08 已登记退场 · 名仍在表 ⇒ 盾随登记存续）
	"start-zerg-core":   {"write", dangerD2}, // scripts/build/start-zerg-core.sh（写仓外 plist + chmod 仓内件）
	"zerg-swap-core":    {"write", dangerD2}, // scripts/build/zerg-swap-core.sh（真写 repo bin/zerg-core + 委派 build-all；2026-10-09 已登记退场 DEV-2055 · 名仍在表 ⇒ 盾随登记存续）
	"zerg-upgrade":      {"write", dangerD2}, // scripts/build/zerg-upgrade.sh（真写 repo bin/ 换装）
	"build-wall-mutate": {"write", dangerD2}, // scripts/build/build-wall-mutate.py（真改仓内源件再还原）
}
