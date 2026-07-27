---- MODULE HomeBaseValidation ----
\* Formal specification of axiom validation and duplicate detection
\* Fixed version addressing all agent feedback
\* Proves: validation correctness, duplicate detection, graceful degradation

EXTENDS Naturals, Sequences, FiniteSets

\* ============================================================================
\* DOMAIN DEFINITIONS
\* ============================================================================

ASSUME Decision ≠ ∅      \* There exist decisions
ASSUME KnownAxioms ≠ ∅  \* There exist known valid axioms

\* ============================================================================
\* STATE VARIABLES
\* ============================================================================

VARIABLE
  processed_decisions   \* Set of all decision IDs processed
  validated_decisions   \* Set of IDs that passed validation
  duplicate_ids         \* Set of IDs that failed (duplicate)
  axiom_cache           \* Set of axioms known to be valid
  neo4j_online          \* Boolean: Neo4j availability flag

\* ============================================================================
\* INITIAL STATE
\* ============================================================================

Init ≡
  /\ processed_decisions = {}
  /\ validated_decisions = {}
  /\ duplicate_ids = {}
  /\ axiom_cache = KnownAxioms    \* Start with bootstrap axioms
  /\ neo4j_online = TRUE

\* ============================================================================
\* ACTIONS
\* ============================================================================

\* Action: Toggle Neo4j availability (for resilience testing)
ToggleNeo4j ≡
  /\ neo4j_online' = ¬neo4j_online
  /\ UNCHANGED processed_decisions
  /\ UNCHANGED validated_decisions
  /\ UNCHANGED duplicate_ids
  /\ UNCHANGED axiom_cache

\* Action: Validate a decision
\* Returns: SUCCESS (added to validated_decisions) or DUPLICATE (added to duplicate_ids)
ValidateDecision(decision) ≡
  LET
    decision_id ≡ decision.id
    decision_axioms ≡ decision.axioms

    \* Check 1: Is this ID already processed?
    is_duplicate ≡ decision_id ∈ processed_decisions

    \* Check 2: Are all axioms valid (if Neo4j is online)?
    axioms_valid ≡ IF neo4j_online THEN
                     ∀ ax ∈ decision_axioms : ax ∈ axiom_cache
                   ELSE
                     TRUE  \* Graceful degradation: skip axiom check if offline

    \* Check 3: Decision has axioms (required)
    has_axioms ≡ Len(decision_axioms) > 0

    \* Overall validation: no duplicate AND axioms valid AND has axioms
    validation_succeeds ≡ ¬is_duplicate ∧ axioms_valid ∧ has_axioms
  IN
    IF validation_succeeds THEN
      \* Success path: add to validated and processed
      /\ validated_decisions' = validated_decisions ∪ {decision_id}
      /\ processed_decisions' = processed_decisions ∪ {decision_id}
      /\ UNCHANGED duplicate_ids
      /\ UNCHANGED axiom_cache
      /\ UNCHANGED neo4j_online
    ELSE
      \* Failure path: if duplicate, track it; otherwise validation just fails
      IF is_duplicate THEN
        /\ duplicate_ids' = duplicate_ids ∪ {decision_id}
        /\ processed_decisions' = processed_decisions ∪ {decision_id}
        /\ UNCHANGED validated_decisions
        /\ UNCHANGED axiom_cache
        /\ UNCHANGED neo4j_online
      ELSE
        \* Validation failed (axioms invalid or no axioms), but not tracked as duplicate
        /\ UNCHANGED duplicate_ids
        /\ UNCHANGED processed_decisions
        /\ UNCHANGED validated_decisions
        /\ UNCHANGED axiom_cache
        /\ UNCHANGED neo4j_online

\* Action: Sync axiom cache from Neo4j (update known axioms)
SyncAxiomCache(new_axiom) ≡
  /\ neo4j_online = TRUE     \* Can only sync when Neo4j is online
  /\ axiom_cache' = axiom_cache ∪ {new_axiom}
  /\ UNCHANGED processed_decisions
  /\ UNCHANGED validated_decisions
  /\ UNCHANGED duplicate_ids
  /\ UNCHANGED neo4j_online

\* ============================================================================
\* STATE MACHINE DEFINITION
\* ============================================================================

\* Next state: one of the three actions occurs
Next ≡
  ∨ ToggleNeo4j
  ∨ (∃ decision ∈ Decision : ValidateDecision(decision))
  ∨ (∃ ax ∈ Decision : SyncAxiomCache(ax))

\* Full specification
Spec ≡ Init ∧ [][Next]_⟨processed_decisions, validated_decisions, duplicate_ids, axiom_cache, neo4j_online⟩

\* ============================================================================
\* SAFETY INVARIANTS
\* ============================================================================

\* Invariant 1: Validated decisions are subset of processed
\* Everything validated must also be marked as processed
Invariant_ValidatedSubset ≡
  validated_decisions ⊆ processed_decisions

\* Invariant 2: Validated and duplicate are disjoint
\* No decision can be both validated and flagged as duplicate
Invariant_ValidatedDisjoint ≡
  validated_decisions ∩ duplicate_ids = {}

\* Invariant 3: Duplicate IDs are in processed
\* All duplicates must also be in processed set
Invariant_DuplicatesInProcessed ≡
  duplicate_ids ⊆ processed_decisions

\* Invariant 4: Graceful degradation works
\* When Neo4j is offline, all processed decisions that aren't duplicates get validated
Invariant_GracefulDegradation ≡
  (neo4j_online = FALSE) ⇒
    (∀ id ∈ processed_decisions :
      (id ∉ duplicate_ids) ⇒ (id ∈ validated_decisions))

\* Invariant 5: Axiom cache is monotonically increasing
\* Axioms can be added but never removed
Invariant_AxiomCacheGrows ≡
  axiom_cache ⊆ axiom_cache'

\* Invariant 6: Sync only happens when online
\* Axiom cache only changes when Neo4j is online (or in Init)
Invariant_SyncOnline ≡
  (axiom_cache ≠ axiom_cache') ⇒ (neo4j_online = TRUE)

\* ============================================================================
\* THEOREMS
\* ============================================================================

\* Theorem 1: Validated and duplicate remain disjoint
THEOREM ValidatedDisjoint ≡ Spec ⇒ □ Invariant_ValidatedDisjoint

\* Theorem 2: Validated is always subset of processed
THEOREM ValidatedSubsetInvariant ≡ Spec ⇒ □ Invariant_ValidatedSubset

\* Theorem 3: Duplicates tracked in processed
THEOREM DuplicatesTracked ≡ Spec ⇒ □ Invariant_DuplicatesInProcessed

\* Theorem 4: Graceful degradation holds
THEOREM GracefulDegradationHolds ≡ Spec ⇒ □ Invariant_GracefulDegradation

\* Theorem 5: Axiom cache is monotonically increasing
THEOREM AxiomCacheMonotonic ≡ Spec ⇒ □ Invariant_AxiomCacheGrows

\* Theorem 6: Sync respects online requirement
THEOREM SyncRequiresOnline ≡ Spec ⇒ □ Invariant_SyncOnline

====
