/-
  Orbit.lean — Formal specification of the HomeBase / Orbit execution graph.

  STATUS:    DRAFT — for captain review, then LOCK.
  AUTHORITY: SYSTEM-DESIGN.md §"State Machine: The Five Moves" (lines 235-290)
             and §"Strict Escalation Protocol" (lines 157-231).
  ROLE:      This file is the mathematical contract. Every theorem stated here
             becomes a proof obligation the Dafny implementation must discharge.

  ------------------------------------------------------------------------------
  DEPENDENCIES

  Lean 4 (v4.32.1) + Mathlib (v4.32.1). Build with `lake build` from
  verification/lean/orbit. Mathlib supplies `List.IsPrefix` (`<+:`),
  `List.Pairwise`, `Prod.Lex` and the well-foundedness machinery used by the
  termination argument. All unproven obligations are `sorry` by design: this is
  a specification, and the proofs are the next ticket.
-/

import Mathlib

-- Field and constructor lists in this file are column-aligned deliberately: the
-- spec is read by humans reviewing a contract, not by Mathlib's CI.
set_option linter.style.whitespace false
set_option linter.style.longLine false
set_option linter.style.emptyLine false

/-

  ------------------------------------------------------------------------------
  ITEMS REQUIRING CAPTAIN ADJUDICATION BEFORE LOCK

  D1. STATE COUNT. The task brief says "5 states" but enumerates 6, and
      SYSTEM-DESIGN.md line 240-246 enumerates 6 (S0..S5, COMPLETE included).
      This spec models SIX constructors. COMPLETE is terminal, not a "move".
      "Five moves" = five transitions out of the five non-terminal states.

  D2. MEANING OF RECOVERY ATTEMPT 2. SYSTEM-DESIGN.md line 266 says
      "Attempt 1 or 2 recovers the failure" (attempt 2 may return to REPEAT),
      but lines 175-181 define Attempt 2 normatively as
      "Escalation preparation: collect full history", whose success leads to
      ESCALATE, not REPEAT. These are contradictory. This spec implements the
      STRICT reading (lines 175-181): attempt 1 is the only attempt that can
      recover; attempt 2 always terminates in ESCALATE. Consequence: a failed
      EXECUTE that attempt 1 cannot fix ALWAYS reaches a human. Captain must
      confirm this reading before lock.

  D3. ADDENDUM — PLAN with an empty queue. The design table has no S0 → S5 edge,
      so an empty work plan would deadlock in PLAN. This spec adds
      `Step.planComplete`. Captain must approve the added edge.

  D4. ADDENDUM — escalation expiry. The design table gives ESCALATE only the
      edges S3 → S4 (human approves) and S3 → S5 (human rejects). Both require a
      human. Therefore, as designed, the machine CAN BLOCK FOREVER in ESCALATE
      and the termination theorem is FALSE. This spec adds `Step.escalateExpire`
      (ESCALATE → COMPLETE, status EXPIRED, no human required), which is the
      minimum edge that makes deadlock-freedom and termination provable.
      SYSTEM-DESIGN.md already lists StatusExpired, so this is believed to be an
      omission in the transition table rather than a design change — but it is a
      new edge and requires captain approval.

  D5. HUMAN REJECTION HALTS THE WHOLE RUN. Per design line 277-279, S3 → S5 goes
      to COMPLETE, abandoning any decisions still in the queue. Modeled as
      specified. Captain should confirm this is intended rather than S3 → S4.
-/

namespace Orbit

/-! ============================================================================
    SECTION 1. IDENTIFIERS AND CRYPTOGRAPHIC PRIMITIVES

    Identifiers are wrapped, not bare Strings, so the type system forbids
    passing a decision id where an axiom id is required (mirrors the sealed-type
    discipline in SYSTEM-DESIGN.md §"Type System Design").
    ============================================================================ -/

structure DecisionId where
  value : String
  deriving DecidableEq, Repr

structure EscalationId where
  value : String
  deriving DecidableEq, Repr

structure AxiomId where
  value : String
  deriving DecidableEq, Repr

structure AgentId where
  value : String
  deriving DecidableEq, Repr

/-- A signature value. Only the signer may construct one in the implementation;
    in the spec its provenance is constrained by `verifySignature`. -/
structure Signature where
  value : String
  deriving DecidableEq, Repr

structure PublicKey where
  value : String
  deriving DecidableEq, Repr

/-! ============================================================================
    SECTION 2. CORE DOMAIN TYPES: Decision, ToolCall, Escalation
    ============================================================================ -/

/-- An unsigned decision. Every field is immutable after construction (I1).
    `axioms` must be non-empty and fully grounded in the Neo4j ontology (I6). -/
structure Decision where
  id         : DecisionId
  statement  : String
  axioms     : List AxiomId
  evidence   : String
  decidedBy  : AgentId
  recordedAt : Nat            -- unix millis
  deriving DecidableEq, Repr

/-- A decision plus the signature that binds it to a signer (I4, I5). -/
structure SignedDecision where
  payload   : Decision
  signer    : PublicKey
  signature : Signature
  deriving DecidableEq, Repr

/-- Canonical encoding of a decision payload for signing/verification. -/
opaque encode : Decision → String

/-- Signature verification. Opaque; its properties are given as axioms below. -/
opaque verifySignature : PublicKey → String → Signature → Bool

/-- Content hash of a ledger entry, chaining prevHash with the signed decision.
    Opaque; collision resistance is assumed (see `hash_injective`). -/
opaque hashEntry : String → SignedDecision → String

/-- Genesis chain anchor. -/
def genesisHash : String := "GENESIS"

/-- A decision is `Signed` when its signature verifies over its canonical
    encoding under its declared signer key. This is invariant I4's atom. -/
def SignedOK (sd : SignedDecision) : Prop :=
  verifySignature sd.signer (encode sd.payload) sd.signature = true

/-- A tool call intercepted before execution. -/
structure ToolCall where
  tool        : String
  args        : List (String × String)
  caller      : AgentId
  citedAxioms : List AxiomId
  deriving DecidableEq, Repr

/-- The ontology contract for one tool, as stored in Neo4j. -/
structure ToolContract where
  tool           : String
  requiredAxioms : List AxiomId
  allowedCallers : List AgentId
  deriving DecidableEq, Repr

/-- A snapshot of the Neo4j knowledge graph: the set of axioms that exist and
    the tool contracts the Interceptor enforces. -/
structure Ontology where
  axioms        : List AxiomId
  toolContracts : List ToolContract
  deriving DecidableEq, Repr

/-- The Interceptor's verdict on a tool call. There is no third option: the
    Interceptor may not "allow with warning". -/
inductive Verdict where
  | allow : Verdict
  | deny  : String → Verdict
  deriving DecidableEq, Repr, Inhabited

/-- A response received from Bridge. Non-repudiation (I5) is decided entirely by
    the signature over `body` under a key that must equal the trusted key. -/
structure BridgeResponse where
  requestId : String
  body      : String
  signer    : PublicKey
  signature : Signature
  deriving DecidableEq, Repr

/-- Bridge response authenticity: right key AND valid signature. -/
def BridgeAuthentic (trusted : PublicKey) (r : BridgeResponse) : Prop :=
  r.signer = trusted ∧ verifySignature r.signer r.body r.signature = true

/-- Enumerated causes of EXECUTE failure (SYSTEM-DESIGN.md lines 254-258, plus
    the two security rejections). The set is closed: EXECUTE may not invent a
    failure mode outside this list. -/
inductive Failure where
  | signatureInvalid   : Failure
  | axiomMissing       : AxiomId → Failure
  | neo4jTimeout       : Failure
  | ledgerWriteError   : Failure
  | bridgeForged       : Failure
  | ontologyViolation  : ToolCall → Failure
  deriving DecidableEq, Repr

/-- The recovery actions, fixed in advance. There is no "other". -/
inductive RecoveryAction where
  | technicalRetry             : RecoveryAction   -- Attempt 1
  | collectHistoryAndEscalate  : RecoveryAction   -- Attempt 2
  deriving DecidableEq, Repr

/-- The bound. Written once, here. Not a parameter of any function that runs
    during EXECUTE — the limit is decided before execution begins. -/
def maxRecoveryAttempts : Nat := 2

/-- The recovery protocol as a total function from attempt index to the action
    that attempt is permitted to take. Attempt 3 and beyond map to `none`:
    there is no action defined for them, so no implementation can take one. -/
def recoveryAction : Nat → Option RecoveryAction
  | 1 => some RecoveryAction.technicalRetry
  | 2 => some RecoveryAction.collectHistoryAndEscalate
  | _ => none

/-- One recorded recovery attempt. Append-only history handed to ESCALATE. -/
structure Attempt where
  index     : Nat                    -- 0 = the original EXECUTE, 1 and 2 = recovery
  action    : Option RecoveryAction  -- none only for index 0
  succeeded : Bool
  failure   : Option Failure
  at_       : Nat
  deriving DecidableEq, Repr

/-- Escalation status. Sealed enum, mirrors SYSTEM-DESIGN.md line 341-347. -/
inductive EscalationStatus where
  | pending  : EscalationStatus
  | approved : EscalationStatus
  | rejected : EscalationStatus
  | expired  : EscalationStatus
  deriving DecidableEq, Repr

/-- An escalation record. `status` is the only mutable field, and it may leave
    `pending` exactly once (I2). -/
structure Escalation where
  id          : EscalationId
  decision    : Decision
  history     : List Attempt
  status      : EscalationStatus
  approvedBy  : Option AgentId
  approvedAt  : Option Nat
  deriving DecidableEq, Repr

/-- An immutable ledger entry. `seq` and `prevHash` fix its position; `hash`
    chains it to its predecessor. -/
structure LedgerEntry where
  seq      : Nat
  decision : SignedDecision
  prevHash : String
  hash     : String
  bridge   : Option BridgeResponse
  deriving DecidableEq, Repr

/-- The ledger is an ordered list of entries. Append-only by construction of the
    step relation, proved by theorem `I1_ledger_is_prefix_monotone`. -/
abbrev Ledger := List LedgerEntry

/-- One approval event: which escalation, by whom, at what logical time. This
    log is append-only and is the witness for I2. -/
structure ApprovalRecord where
  escalation : EscalationId
  approver   : AgentId
  at_        : Nat
  deriving DecidableEq, Repr

/-! ============================================================================
    SECTION 3. GROUNDING (I6) AND THE INTERCEPTOR CONSTRAINT
    ============================================================================ -/

/-- I6 atom: a decision is grounded when it cites at least one axiom and every
    cited axiom exists in the Neo4j snapshot. A fake axiom cannot be grounded. -/
def Grounded (o : Ontology) (d : Decision) : Prop :=
  d.axioms ≠ [] ∧ ∀ a ∈ d.axioms, a ∈ o.axioms

/-- The Interceptor's admission rule for a tool call, stated positively.
    A call is admissible iff the ontology contains a contract for the tool,
    the caller is on that contract's allow-list, every axiom the contract
    requires is cited by the call, and every cited axiom exists in Neo4j. -/
def InterceptorAllows (o : Ontology) (tc : ToolCall) : Prop :=
  ∃ c ∈ o.toolContracts,
    c.tool = tc.tool ∧
    tc.caller ∈ c.allowedCallers ∧
    (∀ a ∈ c.requiredAxioms, a ∈ tc.citedAxioms) ∧
    (∀ a ∈ tc.citedAxioms, a ∈ o.axioms)

/-- The Interceptor as a decision procedure must agree with `InterceptorAllows`.
    This is the soundness/completeness contract the Dafny interceptor satisfies. -/
opaque intercept : Ontology → ToolCall → Verdict

/-! ============================================================================
    SECTION 4. THE FIVE MOVES: STATES AND MACHINE CONFIGURATION
    ============================================================================ -/

/-- The graph states. COMPLETE is terminal (see `complete_is_terminal`). -/
inductive State where
  | PLAN     : State   -- S0 : lock the workflow
  | EXECUTE  : State   -- S1 : record, sign, index one decision
  | RECOVER  : State   -- S2 : bounded recovery protocol
  | ESCALATE : State   -- S3 : hand to human, no further autonomy
  | REPEAT   : State   -- S4 : continue to next decision
  | COMPLETE : State   -- S5 : terminal
  deriving DecidableEq, Repr

/-- The full machine configuration.

    Durability split (I3): `ledger`, `escalations` and `approvals` are durable
    (fsync'd before the transition is acknowledged). `buffer`, `current`,
    `history` and `attempts` are volatile and are destroyed by a crash. -/
structure Config where
  state       : State
  current     : Option Decision          -- decision under execution
  queue       : List Decision            -- remaining planned work
  attempts    : Nat                      -- recovery attempts consumed for `current`
  history     : List Attempt             -- attempt history for `current`
  ledger      : Ledger                   -- DURABLE
  buffer      : List LedgerEntry         -- VOLATILE, not yet fsync'd
  escalations : List Escalation          -- DURABLE
  approvals   : List ApprovalRecord      -- DURABLE, append-only
  executed    : List ToolCall            -- tool calls actually performed
  toolLog     : List (ToolCall × Verdict)-- every call the Interceptor ruled on
  bridgeSeen  : List BridgeResponse      -- bridge responses accepted as genuine
  ontology    : Ontology                 -- Neo4j snapshot
  bridgeKey   : PublicKey                -- trusted Bridge public key
  clock       : Nat

/-! ---------------------------------------------------------------------------
    Helper functions used in transition postconditions. Total, no `sorry`.
    --------------------------------------------------------------------------- -/

def lastHash (l : Ledger) : String :=
  match l.getLast? with
  | some e => e.hash
  | none   => genesisHash

/-- Append one entry to the ledger, fixing its sequence number and chain link. -/
def appendEntry (l : Ledger) (sd : SignedDecision) (br : Option BridgeResponse) : Ledger :=
  l ++ [{ seq      := l.length
        , decision := sd
        , prevHash := lastHash l
        , hash     := hashEntry (lastHash l) sd
        , bridge   := br }]

/-- Mark an escalation approved. Only `pending` records are touched; this makes
    a second approval a no-op at the data level, while the step relation
    forbids it outright (I2). -/
def approveIn (es : List Escalation) (eid : EscalationId) (who : AgentId) (t : Nat)
    : List Escalation :=
  es.map fun e =>
    if e.id = eid ∧ e.status = EscalationStatus.pending then
      { e with status := EscalationStatus.approved, approvedBy := some who, approvedAt := some t }
    else e

/-- Close an escalation with a non-approval terminal status. -/
def closeIn (es : List Escalation) (eid : EscalationId) (s : EscalationStatus)
    : List Escalation :=
  es.map fun e => if e.id = eid ∧ e.status = EscalationStatus.pending then { e with status := s } else e

/-- Record one attempt into the history. -/
def logAttempt (h : List Attempt) (a : Attempt) : List Attempt := h ++ [a]

/-! ============================================================================
    SECTION 5. WELL-FORMEDNESS AND THE STRUCTURAL INVARIANTS

    These predicates are what the step relation must preserve. They are the
    Dafny class invariant.
    ============================================================================ -/

/-- Every ledger entry's signature verifies (I4 atom, lifted to the ledger). -/
def EntryVerified (e : LedgerEntry) : Prop := SignedOK e.decision

/-- The hash chain is intact: sequence numbers are positional and each entry's
    hash is the chained hash of its predecessor's hash and its own payload. -/
def ChainValid : Ledger → Prop
  | [] => True
  | e :: rest =>
      e.seq = 0 ∧ e.prevHash = genesisHash ∧ e.hash = hashEntry genesisHash e.decision ∧
      ChainValidFrom e rest
where
  ChainValidFrom (prev : LedgerEntry) : Ledger → Prop
    | [] => True
    | e :: rest =>
        e.seq = prev.seq + 1 ∧ e.prevHash = prev.hash ∧
        e.hash = hashEntry prev.hash e.decision ∧ ChainValidFrom e rest

/-- No decision id appears twice in the ledger. -/
def UniqueDecisionIds (l : Ledger) : Prop :=
  (l.map (fun e => e.decision.payload.id)).Pairwise (· ≠ ·)

/-- I2 atom: no escalation id appears twice in the approval log. -/
def UniqueApprovals (c : Config) : Prop :=
  (c.approvals.map (fun a => a.escalation)).Pairwise (· ≠ ·)

/-- Every entry ever committed is verified and grounded (I4 + I6 at rest). -/
def LedgerSound (c : Config) : Prop :=
  ∀ e ∈ c.ledger, EntryVerified e ∧ Grounded c.ontology e.decision.payload

/-- Every executed tool call was admitted by the Interceptor. -/
def ToolLogSound (c : Config) : Prop :=
  (∀ tc ∈ c.executed, InterceptorAllows c.ontology tc) ∧
  (∀ tc ∈ c.executed, (tc, Verdict.allow) ∈ c.toolLog)

/-- Every bridge response the machine acted on is authentic (I5 at rest). -/
def BridgeLogSound (c : Config) : Prop :=
  ∀ r ∈ c.bridgeSeen, BridgeAuthentic c.bridgeKey r

/-- The state-dependent shape invariant. This is what makes progress provable:
    each non-terminal state carries exactly the data its outgoing edges need. -/
def StateShape (c : Config) : Prop :=
  (c.state = State.PLAN     → c.current = none ∧ c.attempts = 0 ∧ c.history = []) ∧
  (c.state = State.EXECUTE  → c.current.isSome = true ∧ c.attempts = 0) ∧
  (c.state = State.RECOVER  → c.current.isSome = true ∧ c.attempts < maxRecoveryAttempts) ∧
  (c.state = State.ESCALATE → c.attempts = maxRecoveryAttempts ∧
                              ∃ e ∈ c.escalations, e.status = EscalationStatus.pending) ∧
  (c.state = State.REPEAT   → c.attempts ≤ maxRecoveryAttempts) ∧
  (c.state = State.COMPLETE → c.buffer = [])

/-- The full machine invariant. -/
def WellFormed (c : Config) : Prop :=
  c.attempts ≤ maxRecoveryAttempts ∧
  StateShape c ∧
  ChainValid c.ledger ∧
  UniqueDecisionIds c.ledger ∧
  UniqueApprovals c ∧
  LedgerSound c ∧
  ToolLogSound c ∧
  BridgeLogSound c

/-- A legal initial configuration: PLAN, nothing in flight, nothing buffered. -/
def Init (c : Config) : Prop :=
  c.state = State.PLAN ∧ c.current = none ∧ c.attempts = 0 ∧
  c.history = [] ∧ c.buffer = [] ∧ WellFormed c

/-! ============================================================================
    SECTION 6. THE TRANSITION RELATION

    One constructor per edge in SYSTEM-DESIGN.md lines 248-288, plus the two
    addenda D3 and D4. Preconditions are the constructor's hypotheses;
    postconditions are the shape of the resulting `Config`. There is no other
    way to change a `Config`: anything not listed here cannot happen.
    ============================================================================ -/

inductive Step : Config → Config → Prop where

  /-- S0 → S1. PLAN locks the workflow and hands the first decision to EXECUTE.
      Preconditions: in PLAN, the queue is non-empty. Postcondition: the plan is
      fixed — `queue` is the locked tail and cannot be extended by EXECUTE. -/
  | planLock
      (c : Config) (d : Decision) (rest : List Decision)
      (hs : c.state = State.PLAN)
      (hq : c.queue = d :: rest) :
      Step c { c with state := State.EXECUTE, current := some d, queue := rest
                    , attempts := 0, history := [] }

  /-- ADDENDUM D3. S0 → S5. Empty plan completes immediately. -/
  | planComplete
      (c : Config)
      (hs : c.state = State.PLAN)
      (hq : c.queue = []) :
      Step c { c with state := State.COMPLETE, buffer := [] }

  /-- S1 → S4. EXECUTE succeeds: record, sign, index — all three, in order, with
      no step skipped (SYSTEM-DESIGN.md Mistake 2).
      Preconditions, all mandatory:
        * the signed decision carries the decision under execution (I1),
        * its signature verifies (I4),
        * it is grounded in the Neo4j snapshot (I6),
        * its id is not already in the ledger (I2 for decisions),
        * every tool call used was admitted by the Interceptor,
        * any Bridge response consumed is authentic (I5),
        * the entry is fsync'd into `ledger`, not left in `buffer` (I3). -/
  | execCommit
      (c : Config) (d : Decision) (sd : SignedDecision)
      (calls : List ToolCall) (br : Option BridgeResponse)
      (hs   : c.state = State.EXECUTE)
      (hcur : c.current = some d)
      (hbind: sd.payload = d)
      (hsig : SignedOK sd)
      (hgrd : Grounded c.ontology d)
      (hnew : ∀ e ∈ c.ledger, e.decision.payload.id ≠ d.id)
      (hint : ∀ tc ∈ calls, InterceptorAllows c.ontology tc)
      (hbr  : ∀ r, br = some r → BridgeAuthentic c.bridgeKey r) :
      Step c { c with state    := State.REPEAT
                    , current  := none
                    , ledger   := appendEntry c.ledger sd br
                    , buffer   := []
                    , executed := c.executed ++ calls
                    , toolLog  := c.toolLog ++ calls.map (fun tc => (tc, Verdict.allow))
                    , bridgeSeen := match br with | some r => c.bridgeSeen ++ [r] | none => c.bridgeSeen
                    , history  := logAttempt c.history
                        { index := 0, action := none, succeeded := true, failure := none, at_ := c.clock } }

  /-- S1 → S2. EXECUTE fails for one of the enumerated causes. EXECUTE does NOT
      decide what to do about it: it only reports the failure and moves to
      RECOVER. The ledger is untouched (I1, I4). -/
  | execFail
      (c : Config) (d : Decision) (f : Failure)
      (hs   : c.state = State.EXECUTE)
      (hcur : c.current = some d) :
      Step c { c with state    := State.RECOVER
                    , attempts := 0
                    , buffer   := []
                    , history  := logAttempt c.history
                        { index := 0, action := none, succeeded := false
                        , failure := some f, at_ := c.clock } }

  /-- S2 → S4. Recovery ATTEMPT 1 (technical retry) succeeds. The retried write
      must satisfy exactly the same integrity, grounding and uniqueness
      preconditions as `execCommit` — recovery may not lower the bar. -/
  | recoverAttempt1Success
      (c : Config) (d : Decision) (sd : SignedDecision) (br : Option BridgeResponse)
      (hs   : c.state = State.RECOVER)
      (hat  : c.attempts = 0)
      (hact : recoveryAction 1 = some RecoveryAction.technicalRetry)
      (hcur : c.current = some d)
      (hbind: sd.payload = d)
      (hsig : SignedOK sd)
      (hgrd : Grounded c.ontology d)
      (hnew : ∀ e ∈ c.ledger, e.decision.payload.id ≠ d.id)
      (hbr  : ∀ r, br = some r → BridgeAuthentic c.bridgeKey r) :
      Step c { c with state    := State.REPEAT
                    , current  := none
                    , attempts := 1
                    , ledger   := appendEntry c.ledger sd br
                    , buffer   := []
                    , bridgeSeen := match br with | some r => c.bridgeSeen ++ [r] | none => c.bridgeSeen
                    , history  := logAttempt c.history
                        { index := 1, action := some RecoveryAction.technicalRetry
                        , succeeded := true, failure := none, at_ := c.clock } }

  /-- S2 → S2. Recovery ATTEMPT 1 fails. Consumes the attempt. No ledger change. -/
  | recoverAttempt1Fail
      (c : Config) (f : Failure)
      (hs   : c.state = State.RECOVER)
      (hat  : c.attempts = 0)
      (hact : recoveryAction 1 = some RecoveryAction.technicalRetry) :
      Step c { c with attempts := 1
                    , history  := logAttempt c.history
                        { index := 1, action := some RecoveryAction.technicalRetry
                        , succeeded := false, failure := some f, at_ := c.clock } }

  /-- S2 → S3. Recovery ATTEMPT 2 is escalation preparation (decision D2). It
      collects the full history and hands it to a human. Its outcome does not
      change the destination: whether history collection succeeds or fails, the
      next state is ESCALATE. This is the edge that makes "no Attempt 3"
      structural rather than a runtime check. -/
  | recoverAttempt2Escalate
      (c : Config) (d : Decision) (eid : EscalationId) (collected : Bool)
      (hs   : c.state = State.RECOVER)
      (hat  : c.attempts = 1)
      (hact : recoveryAction 2 = some RecoveryAction.collectHistoryAndEscalate)
      (hcur : c.current = some d)
      (hfresh : ∀ e ∈ c.escalations, e.id ≠ eid) :
      Step c { c with state    := State.ESCALATE
                    , attempts := maxRecoveryAttempts
                    , history  := logAttempt c.history
                        { index := 2, action := some RecoveryAction.collectHistoryAndEscalate
                        , succeeded := collected, failure := none, at_ := c.clock }
                    , escalations := c.escalations ++
                        [{ id := eid, decision := d
                         , history := logAttempt c.history
                             { index := 2, action := some RecoveryAction.collectHistoryAndEscalate
                             , succeeded := collected, failure := none, at_ := c.clock }
                         , status := EscalationStatus.pending
                         , approvedBy := none, approvedAt := none }] }

  /-- S3 → S4. A human approves. Preconditions enforcing I2 and I5:
        * the escalation exists and is still `pending`,
        * its id does not already appear in the append-only approval log,
        * the approval arrived as an authentic Bridge response.
      ESCALATE itself decides nothing; it only records what the human decided. -/
  | escalateApprove
      (c : Config) (e : Escalation) (who : AgentId) (r : BridgeResponse)
      (hs    : c.state = State.ESCALATE)
      (hmem  : e ∈ c.escalations)
      (hpend : e.status = EscalationStatus.pending)
      (honce : ∀ a ∈ c.approvals, a.escalation ≠ e.id)
      (hauth : BridgeAuthentic c.bridgeKey r) :
      Step c { c with state       := State.REPEAT
                    , current     := none
                    , escalations := approveIn c.escalations e.id who c.clock
                    , approvals   := c.approvals ++
                        [{ escalation := e.id, approver := who, at_ := c.clock }]
                    , bridgeSeen  := c.bridgeSeen ++ [r] }

  /-- S3 → S5. A human rejects. The run ends (decision D5). No approval is
      logged, so the escalation can never later be counted as approved. -/
  | escalateReject
      (c : Config) (e : Escalation) (r : BridgeResponse)
      (hs    : c.state = State.ESCALATE)
      (hmem  : e ∈ c.escalations)
      (hpend : e.status = EscalationStatus.pending)
      (hauth : BridgeAuthentic c.bridgeKey r) :
      Step c { c with state       := State.COMPLETE
                    , current     := none
                    , buffer      := []
                    , escalations := closeIn c.escalations e.id EscalationStatus.rejected
                    , bridgeSeen  := c.bridgeSeen ++ [r] }

  /-- ADDENDUM D4. S3 → S5 by expiry. No human required, therefore ESCALATE can
      never block forever. Logs no approval. -/
  | escalateExpire
      (c : Config) (e : Escalation)
      (hs    : c.state = State.ESCALATE)
      (hmem  : e ∈ c.escalations)
      (hpend : e.status = EscalationStatus.pending) :
      Step c { c with state       := State.COMPLETE
                    , current     := none
                    , buffer      := []
                    , escalations := closeIn c.escalations e.id EscalationStatus.expired }

  /-- S4 → S1. More work in the locked plan. Attempt counter resets for the new
      decision; the previous decision's history is closed out. -/
  | repeatNext
      (c : Config) (d : Decision) (rest : List Decision)
      (hs : c.state = State.REPEAT)
      (hq : c.queue = d :: rest) :
      Step c { c with state := State.EXECUTE, current := some d, queue := rest
                    , attempts := 0, history := [] }

  /-- S4 → S5. Plan exhausted. -/
  | repeatDone
      (c : Config)
      (hs : c.state = State.REPEAT)
      (hq : c.queue = []) :
      Step c { c with state := State.COMPLETE, current := none, buffer := [] }

/-- Reflexive-transitive closure of `Step`. -/
inductive Steps : Config → Config → Prop where
  | refl (c : Config) : Steps c c
  | tail {a b d : Config} : Steps a b → Step b d → Steps a d

/-- Reachability from a legal initial configuration. -/
def Reachable (c : Config) : Prop := ∃ i, Init i ∧ Steps i c

/-! ============================================================================
    SECTION 7. CRASH MODEL (for I3)
    ============================================================================ -/

/-- A process crash. Volatile state is destroyed; durable state survives. The
    machine restarts in PLAN with whatever work remains. This is the ONLY model
    of a crash: an implementation that loses `ledger`, `escalations` or
    `approvals` across a crash does not refine this specification. -/
def crash (c : Config) : Config :=
  { c with state    := State.PLAN
         , current  := none
         , attempts := 0
         , history  := []
         , buffer   := [] }

/-! ============================================================================
    SECTION 8. CRYPTOGRAPHIC ASSUMPTIONS

    These are ASSUMPTIONS about Ed25519 and the hash, not obligations on the
    implementation. They are stated as `axiom` so that any theorem depending on
    them is visibly conditional on the crypto being sound.
    ============================================================================ -/

/-- Only the holder of the matching private key can produce a verifying
    signature. Formalized as: verification determines the signer key, so two
    different keys cannot both verify the same message/signature pair. -/
axiom verify_key_binding :
  ∀ (m : String) (s : Signature) (k₁ k₂ : PublicKey),
    verifySignature k₁ m s = true → verifySignature k₂ m s = true → k₁ = k₂

/-- Signatures bind the exact message: a verifying signature over `m` does not
    verify over any different message. -/
axiom verify_message_binding :
  ∀ (m m' : String) (s : Signature) (k : PublicKey),
    m ≠ m' → verifySignature k m s = true → verifySignature k m' s = false

/-- Canonical encoding is injective: distinct decisions have distinct encodings,
    so a signature cannot be transplanted between decisions. -/
axiom encode_injective :
  ∀ (d₁ d₂ : Decision), encode d₁ = encode d₂ → d₁ = d₂

/-- Chained hashing is collision resistant on the arguments that matter. -/
axiom hash_injective :
  ∀ (p₁ p₂ : String) (s₁ s₂ : SignedDecision),
    hashEntry p₁ s₁ = hashEntry p₂ s₂ → p₁ = p₂ ∧ s₁ = s₂

/-- The implemented Interceptor is sound and complete w.r.t. the ontology rule. -/
axiom intercept_correct :
  ∀ (o : Ontology) (tc : ToolCall),
    (intercept o tc = Verdict.allow ↔ InterceptorAllows o tc)

/-! ============================================================================
    SECTION 9. INVARIANT I1 — IMMUTABILITY

    "Decisions cannot be modified after write."
    ============================================================================ -/

/-- I1.a. The ledger only ever grows at the tail: the old ledger is a prefix of
    the new one after any transition. -/
theorem I1_ledger_is_prefix_monotone :
    ∀ c c', Step c c' → c.ledger <+: c'.ledger := by
  sorry

/-- I1.b. No committed entry is ever removed. -/
theorem I1_entries_persist :
    ∀ c c' e, Step c c' → e ∈ c.ledger → e ∈ c'.ledger := by
  sorry

/-- I1.c. No committed entry is ever rewritten: every index that existed before
    the transition holds the identical entry after it. -/
theorem I1_entries_are_positionally_stable :
    ∀ c c' i, Step c c' → i < c.ledger.length →
      c'.ledger[i]? = c.ledger[i]? := by
  sorry

/-- I1.d. Reachable form: for any reachable configuration, every prefix of the
    ledger observed earlier in the run is still a prefix. -/
theorem I1_immutability :
    ∀ a b, Steps a b → a.ledger <+: b.ledger := by
  sorry

/-- I1.e. Tamper detection: an entry whose payload has been altered fails
    verification, so it cannot be mistaken for a committed entry. Depends on the
    crypto assumptions of Section 8. -/
theorem I1_tamper_is_detectable :
    ∀ (e : LedgerEntry) (d' : Decision),
      EntryVerified e → d' ≠ e.decision.payload →
      verifySignature e.decision.signer (encode d') e.decision.signature = false := by
  sorry

/-! ============================================================================
    SECTION 10. INVARIANT I2 — UNIQUENESS

    "Escalations can be approved at most once."
    ============================================================================ -/

/-- I2.a. The approval log never contains the same escalation id twice. -/
theorem I2_approvals_unique :
    ∀ c c', WellFormed c → Step c c' → UniqueApprovals c' := by
  sorry

/-- I2.b. Reachable form: at most one approval per escalation, ever. -/
theorem I2_uniqueness :
    ∀ c, Reachable c → UniqueApprovals c := by
  sorry

/-- I2.c. Approval requires a pending escalation, so an already-approved,
    rejected or expired escalation cannot be approved. -/
theorem I2_approve_requires_pending :
    ∀ c c' (e : Escalation),
      Step c c' → e ∈ c.escalations →
      (∃ a ∈ c'.approvals, a.escalation = e.id) →
      (∀ a ∈ c.approvals, a.escalation ≠ e.id) →
      e.status = EscalationStatus.pending := by
  sorry

/-- I2.d. Counting form: for every reachable configuration and every escalation
    id, the number of approval records bearing that id is at most one. -/
theorem I2_at_most_one_approval :
    ∀ c, Reachable c →
      ∀ (eid : EscalationId) (a₁ a₂ : ApprovalRecord),
        a₁ ∈ c.approvals → a₂ ∈ c.approvals →
        a₁.escalation = eid → a₂.escalation = eid → a₁ = a₂ := by
  sorry

/-- I2.e. Decision ids are unique in the ledger — the same decision is never
    recorded twice, including via the recovery path. -/
theorem I2_decision_ids_unique :
    ∀ c, Reachable c → UniqueDecisionIds c.ledger := by
  sorry

/-! ============================================================================
    SECTION 11. INVARIANT I3 — DURABILITY

    "Decisions persist across process crash."
    ============================================================================ -/

/-- I3.a. A crash preserves every durable field exactly. -/
theorem I3_crash_preserves_durable_state :
    ∀ c, (crash c).ledger = c.ledger ∧
         (crash c).escalations = c.escalations ∧
         (crash c).approvals = c.approvals := by
  sorry

/-- I3.b. A crash destroys exactly the volatile fields — nothing buffered is
    silently promoted to durable. -/
theorem I3_crash_clears_volatile_state :
    ∀ c, (crash c).buffer = [] ∧ (crash c).current = none ∧
         (crash c).history = [] ∧ (crash c).attempts = 0 := by
  sorry

/-- I3.c. Acknowledgement implies durability: a decision is only observable as
    recorded once it is in the durable ledger, hence it survives a crash. -/
theorem I3_acknowledged_implies_durable :
    ∀ c c' (d : Decision),
      Step c c' →
      (∃ e ∈ c'.ledger, e.decision.payload.id = d.id) →
      (∃ e ∈ (crash c').ledger, e.decision.payload.id = d.id) := by
  sorry

/-- I3.d. A crashed configuration is still well-formed, so the machine can
    restart from it without repair. -/
theorem I3_crash_preserves_wellformedness :
    ∀ c, WellFormed c → WellFormed (crash c) := by
  sorry

/-- I3.e. Durability is closed under repeated crashes at any point in a run. -/
theorem I3_durability :
    ∀ a b, Steps a b → a.ledger <+: (crash b).ledger := by
  sorry

/-! ============================================================================
    SECTION 12. INVARIANT I4 — INTEGRITY

    "Invalid signatures are rejected."
    ============================================================================ -/

/-- I4.a. Every newly committed entry verifies. There is no transition that
    appends an unverified entry. -/
theorem I4_new_entries_are_verified :
    ∀ c c' e, Step c c' → e ∈ c'.ledger → e ∉ c.ledger → EntryVerified e := by
  sorry

/-- I4.b. Whole-ledger form for any reachable configuration. -/
theorem I4_integrity :
    ∀ c, Reachable c → ∀ e ∈ c.ledger, EntryVerified e := by
  sorry

/-- I4.c. An unsigned or badly-signed decision can never enter the ledger. -/
theorem I4_invalid_signature_never_committed :
    ∀ c c' (sd : SignedDecision),
      Step c c' → ¬ SignedOK sd →
      ∀ e ∈ c'.ledger, e ∉ c.ledger → e.decision ≠ sd := by
  sorry

/-- I4.d. The signature binds the entry to the exact decision payload; the
    signer identity in the entry is the only key that could have produced it. -/
theorem I4_signature_binds_entry :
    ∀ (e : LedgerEntry) (k : PublicKey),
      EntryVerified e →
      verifySignature k (encode e.decision.payload) e.decision.signature = true →
      k = e.decision.signer := by
  sorry

/-- I4.e. The hash chain of a reachable ledger is intact. -/
theorem I4_chain_valid :
    ∀ c, Reachable c → ChainValid c.ledger := by
  sorry

/-! ============================================================================
    SECTION 13. INVARIANT I5 — NON-REPUDIATION

    "Forged Bridge responses are rejected."
    ============================================================================ -/

/-- I5.a. Every Bridge response the machine acts on is authentic. -/
theorem I5_accepted_bridge_responses_are_authentic :
    ∀ c c' r, Step c c' → r ∈ c'.bridgeSeen → r ∉ c.bridgeSeen →
      BridgeAuthentic c.bridgeKey r := by
  sorry

/-- I5.b. Whole-run form. -/
theorem I5_non_repudiation :
    ∀ c, Reachable c → ∀ r ∈ c.bridgeSeen, BridgeAuthentic c.bridgeKey r := by
  sorry

/-- I5.c. A forged response — one not signed by the trusted Bridge key — enables
    no transition at all. -/
theorem I5_forged_bridge_response_enables_nothing :
    ∀ c c' (r : BridgeResponse),
      ¬ BridgeAuthentic c.bridgeKey r →
      Step c c' → r ∉ c'.bridgeSeen ∨ r ∈ c.bridgeSeen := by
  sorry

/-- I5.d. Human approvals in particular are non-repudiable: every approval
    record is backed by an authentic Bridge response accepted in the same step. -/
theorem I5_approvals_backed_by_authentic_bridge :
    ∀ c c' a,
      Step c c' → a ∈ c'.approvals → a ∉ c.approvals →
      ∃ r ∈ c'.bridgeSeen, BridgeAuthentic c.bridgeKey r := by
  sorry

/-! ============================================================================
    SECTION 14. INVARIANT I6 — AXIOM GROUNDING (Neo4j constraint)

    "All decisions cite valid axioms from Neo4j."
    ============================================================================ -/

/-- I6.a. Every newly committed decision is grounded in the Neo4j snapshot. -/
theorem I6_new_entries_are_grounded :
    ∀ c c' e, Step c c' → e ∈ c'.ledger → e ∉ c.ledger →
      Grounded c.ontology e.decision.payload := by
  sorry

/-- I6.b. Whole-ledger form. -/
theorem I6_axiom_grounding :
    ∀ c, Reachable c →
      ∀ e ∈ c.ledger, Grounded c.ontology e.decision.payload := by
  sorry

/-- I6.c. No decision with an empty axiom list is ever committed. -/
theorem I6_no_ungrounded_decision :
    ∀ c, Reachable c → ∀ e ∈ c.ledger, e.decision.payload.axioms ≠ [] := by
  sorry

/-- I6.d. A fabricated axiom — one absent from the Neo4j snapshot — cannot
    appear in any committed decision. -/
theorem I6_fake_axiom_never_cited :
    ∀ c, Reachable c →
      ∀ (a : AxiomId), a ∉ c.ontology.axioms →
        ∀ e ∈ c.ledger, a ∉ e.decision.payload.axioms := by
  sorry

/-! ============================================================================
    SECTION 15. THE INTERCEPTOR CONSTRAINT (ontology checks)
    ============================================================================ -/

/-- INT.a. Every tool call the machine executes was admitted by the Interceptor
    against the Neo4j ontology. -/
theorem INT_no_unvalidated_tool_call :
    ∀ c c' tc, Step c c' → tc ∈ c'.executed → tc ∉ c.executed →
      InterceptorAllows c.ontology tc := by
  sorry

/-- INT.b. Whole-run form. -/
theorem INT_all_executed_calls_allowed :
    ∀ c, Reachable c → ∀ tc ∈ c.executed, InterceptorAllows c.ontology tc := by
  sorry

/-- INT.c. A denied call never executes. -/
theorem INT_denied_call_never_executes :
    ∀ c, Reachable c →
      ∀ tc, intercept c.ontology tc ≠ Verdict.allow → tc ∉ c.executed := by
  sorry

/-- INT.d. Every executed call is auditable: it appears in the tool log with an
    `allow` verdict. -/
theorem INT_execution_is_audited :
    ∀ c, Reachable c → ∀ tc ∈ c.executed, (tc, Verdict.allow) ∈ c.toolLog := by
  sorry

/-- INT.e. A call citing an axiom absent from Neo4j is never admitted. This is
    the link between the Interceptor and I6. -/
theorem INT_ungrounded_call_denied :
    ∀ (o : Ontology) (tc : ToolCall) (a : AxiomId),
      a ∈ tc.citedAxioms → a ∉ o.axioms → ¬ InterceptorAllows o tc := by
  sorry

/-! ============================================================================
    SECTION 16. BOUNDED RECOVERY — EXACTLY TWO ATTEMPTS, NEVER THREE
    ============================================================================ -/

/-- R.a. The attempt counter never exceeds the bound in any reachable state. -/
theorem R_attempts_bounded :
    ∀ c, Reachable c → c.attempts ≤ maxRecoveryAttempts := by
  sorry

/-- R.b. Being in RECOVER means at least one attempt remains. Equivalently:
    the machine is never in RECOVER having already used both attempts. -/
theorem R_recover_has_attempts_left :
    ∀ c, Reachable c → c.state = State.RECOVER → c.attempts < maxRecoveryAttempts := by
  sorry

/-- R.c. There is no third attempt: no transition both starts in RECOVER with
    two attempts consumed and stays in RECOVER. -/
theorem R_no_third_attempt :
    ∀ c c', Reachable c → Step c c' →
      c.state = State.RECOVER → c.attempts = maxRecoveryAttempts → False := by
  sorry

/-- R.d. Exhausting recovery forces ESCALATE — it is the only exit from RECOVER
    once attempt 1 has failed. -/
theorem R_exhaustion_forces_escalation :
    ∀ c c', Step c c' → c.state = State.RECOVER → c.attempts = 1 →
      c'.state = State.ESCALATE ∧ c'.attempts = maxRecoveryAttempts := by
  sorry

/-- R.e. Each attempt increases the counter by exactly one — attempts cannot be
    silently reset to buy more retries while the same decision is in flight. -/
theorem R_attempts_increase_monotonically :
    ∀ c c', Step c c' → c.state = State.RECOVER → c'.state = State.RECOVER →
      c'.attempts = c.attempts + 1 := by
  sorry

/-- R.f. The protocol is fixed in advance: attempt 1 is the technical retry,
    attempt 2 is escalation preparation, and no action is defined beyond two. -/
theorem R_protocol_is_predefined :
    recoveryAction 1 = some RecoveryAction.technicalRetry ∧
    recoveryAction 2 = some RecoveryAction.collectHistoryAndEscalate ∧
    (∀ n, n > maxRecoveryAttempts → recoveryAction n = none) := by
  sorry

/-- R.g. At most two recovery attempts are recorded in the history of any single
    decision. -/
theorem R_history_length_bounded :
    ∀ c, Reachable c →
      (c.history.filter (fun a => a.index != 0)).length ≤ maxRecoveryAttempts := by
  sorry

/-- R.h. Recovery never lowers the integrity bar: an entry committed by the
    recovery path satisfies the same verification and grounding preconditions
    as one committed by EXECUTE. -/
theorem R_recovery_preserves_integrity :
    ∀ c c' e, Step c c' → c.state = State.RECOVER →
      e ∈ c'.ledger → e ∉ c.ledger →
      EntryVerified e ∧ Grounded c.ontology e.decision.payload := by
  sorry

/-! ============================================================================
    SECTION 17. GRAPH STRUCTURE: NO DRIFT, NO HIDDEN EDGES
    ============================================================================ -/

/-- G.a. COMPLETE is terminal: nothing follows it. -/
theorem complete_is_terminal :
    ∀ c c', c.state = State.COMPLETE → ¬ Step c c' := by
  sorry

/-- G.b. PLAN is never re-entered: the plan cannot drift backward
    (SYSTEM-DESIGN.md line 290). -/
theorem plan_never_reentered :
    ∀ c c', Step c c' → c'.state ≠ State.PLAN := by
  sorry

/-- G.c. The work queue never grows: EXECUTE and RECOVER cannot add work that
    PLAN did not lock. -/
theorem queue_never_grows :
    ∀ c c', Step c c' → c'.queue.length ≤ c.queue.length := by
  sorry

/-- G.d. Only PLAN and REPEAT may dequeue work. -/
theorem only_plan_and_repeat_dequeue :
    ∀ c c', Step c c' → c'.queue ≠ c.queue →
      c.state = State.PLAN ∨ c.state = State.REPEAT := by
  sorry

/-- G.e. ESCALATE is not a decision point: it never appends to the ledger. The
    machine cannot record a decision while waiting for a human. -/
theorem escalate_writes_no_decision :
    ∀ c c', Step c c' → c.state = State.ESCALATE → c'.ledger = c.ledger := by
  sorry

/-- G.f. EXECUTE never decides recovery policy: it can only reach REPEAT (on
    success) or RECOVER (on failure). -/
theorem execute_successors :
    ∀ c c', Step c c' → c.state = State.EXECUTE →
      c'.state = State.REPEAT ∨ c'.state = State.RECOVER := by
  sorry

/-- G.g. RECOVER never executes new work: it can only reach REPEAT (attempt 1
    recovered the original decision) or RECOVER or ESCALATE. -/
theorem recover_successors :
    ∀ c c', Step c c' → c.state = State.RECOVER →
      c'.state = State.REPEAT ∨ c'.state = State.RECOVER ∨ c'.state = State.ESCALATE := by
  sorry

/-- G.h. The well-formedness invariant is preserved by every transition. This is
    the Dafny class invariant obligation. -/
theorem step_preserves_wellformed :
    ∀ c c', WellFormed c → Step c c' → WellFormed c' := by
  sorry

/-- G.i. Every reachable configuration is well-formed. -/
theorem reachable_wellformed :
    ∀ c, Reachable c → WellFormed c := by
  sorry

/-! ============================================================================
    SECTION 18. DEADLOCK FREEDOM AND TERMINATION
    ============================================================================ -/

/-- Rank of a state for the termination measure. Strictly decreasing along every
    edge except REPEAT → EXECUTE and PLAN → EXECUTE, which strictly decrease the
    queue length instead. -/
def phaseRank : State → Nat
  | State.PLAN     => 5
  | State.EXECUTE  => 4
  | State.RECOVER  => 3
  | State.ESCALATE => 2
  | State.REPEAT   => 1
  | State.COMPLETE => 0

/-- Lexicographic termination measure:
    (work remaining, phase rank, recovery attempts remaining). -/
def runMeasure (c : Config) : Nat × Nat × Nat :=
  (c.queue.length, phaseRank c.state, maxRecoveryAttempts - c.attempts)

/-- Lexicographic strict order on the measure triple, built from Mathlib's
    `Prod.Lex`. Well-founded because `Nat.lt` is (see `lexLt_wellFounded`). -/
def lexLt : Nat × Nat × Nat → Nat × Nat × Nat → Prop :=
  Prod.Lex (· < ·) (Prod.Lex (· < ·) (· < ·))

/-- T.a. DEADLOCK FREEDOM. Every well-formed configuration that is not COMPLETE
    has at least one enabled transition. Note this depends on addenda D3 and D4:
    without `planComplete` an empty plan deadlocks, and without `escalateExpire`
    the machine blocks in ESCALATE until a human acts. -/
theorem no_deadlock :
    ∀ c, WellFormed c → c.state ≠ State.COMPLETE → ∃ c', Step c c' := by
  sorry

/-- T.b. The measure strictly decreases along every transition. -/
theorem step_decreases_measure :
    ∀ c c', Step c c' → lexLt (runMeasure c') (runMeasure c) := by
  sorry

/-- T.c. `lexLt` is well-founded, so no infinite descending chain exists. -/
theorem lexLt_wellFounded : WellFounded lexLt := by
  sorry

/-- T.d. NO INFINITE RUN. There is no infinite sequence of transitions — in
    particular no unbounded retry loop and no PLAN/EXECUTE cycle. -/
theorem no_infinite_run :
    ∀ (f : Nat → Config), (∀ n, Step (f n) (f (n + 1))) → False := by
  sorry

/-- T.e. TERMINATION. From any well-formed configuration the machine can always
    reach COMPLETE. Combined with T.d (no infinite run) and T.a (no deadlock),
    every run reaches COMPLETE in finitely many steps. -/
theorem termination :
    ∀ c, WellFormed c → ∃ c', Steps c c' ∧ c'.state = State.COMPLETE := by
  sorry

/-- T.f. STRONG TERMINATION. Every maximal run ends in COMPLETE: a run that
    cannot step further is in COMPLETE. -/
theorem stuck_implies_complete :
    ∀ c, WellFormed c → (¬ ∃ c', Step c c') → c.state = State.COMPLETE := by
  sorry

/-- T.g. EXPLICIT STEP BOUND. A run from `i` takes at most 18·(|queue|+1) steps.
    The constant is (max phase rank + 1) · (max attempts + 1) = 6 · 3. This is a
    hard resource bound the implementation must respect. -/
theorem run_length_bounded :
    ∀ (i : Config) (f : Nat → Config) (k : Nat),
      f 0 = i → (∀ n, n < k → Step (f n) (f (n + 1))) →
      k ≤ 18 * (i.queue.length + 1) := by
  sorry

/-! ============================================================================
    SECTION 19. TOP-LEVEL CONTRACT

    The single theorem the Dafny implementation must ultimately refine.
    ============================================================================ -/

/-- All six invariants plus the interceptor constraint, holding simultaneously
    at every reachable configuration. -/
def AllInvariants (c : Config) : Prop :=
  (∀ e ∈ c.ledger, EntryVerified e) ∧                                  -- I4
  (∀ e ∈ c.ledger, Grounded c.ontology e.decision.payload) ∧           -- I6
  UniqueDecisionIds c.ledger ∧                                         -- I1/I2
  UniqueApprovals c ∧                                                  -- I2
  ChainValid c.ledger ∧                                                -- I1
  BridgeLogSound c ∧                                                   -- I5
  ToolLogSound c ∧                                                     -- Interceptor
  c.attempts ≤ maxRecoveryAttempts                                     -- bounded recovery

/-- MASTER THEOREM. Orbit is safe (all invariants hold everywhere reachable),
    live (never deadlocks), and terminating (always reaches COMPLETE), and its
    guarantees survive a crash at any point. -/
theorem orbit_correct :
    ∀ c, Reachable c →
      AllInvariants c ∧
      (c.state ≠ State.COMPLETE → ∃ c', Step c c') ∧
      (∃ c', Steps c c' ∧ c'.state = State.COMPLETE) ∧
      AllInvariants (crash c) := by
  sorry

end Orbit
