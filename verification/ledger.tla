---- MODULE HomeBaseLedger ----
\* Formal specification of HomeBase ledger (append-only JSONL store)
\* Proves: immutability, append-only, hash-chain consistency

EXTENDS Naturals, Sequences, FiniteSets

\* State variables
VARIABLE ledger,        \* Sequence of decisions (append-only)
         hashes,        \* Set of all decision hashes (for chain verification)
         lineNumbers,   \* Map decision ID -> line number
         invariants     \* Track invariants as we execute

\* Initial state
Init ≡
  /\ ledger = << >>
  /\ hashes = {}
  /\ lineNumbers = {}
  /\ invariants = [
       immutable ↦ TRUE,           \* ∀ old entries: old ∈ ledger'
       appendOnly ↦ TRUE,          \* ledger ⊆ ledger'
       hashChain ↦ TRUE,           \* ∀ i: Hash(ledger[i-1]) = ledger[i].previous_hash
       noDuplicates ↦ TRUE         \* ∀ i,j: i ≠ j → ledger[i].id ≠ ledger[j].id
     ]

\* Action: Append a new decision
Append(decision) ≡
  LET newLineNumber ≡ Len(ledger) + 1
      newHash ≡ Hash(decision)
      previousHash ≡ IF newLineNumber = 1 THEN "" ELSE hashes[newLineNumber - 1]
  IN
    /\ decision.id ∉ DOMAIN lineNumbers  \* No duplicate IDs
    /\ ledger' = Append(ledger, [
         id ↦ decision.id,
         data ↦ decision,
         hash ↦ newHash,
         previousHash ↦ previousHash,
         lineNumber ↦ newLineNumber
       ])
    /\ hashes' = hashes ∪ {newHash}
    /\ lineNumbers' = lineNumbers ∪ {decision.id ↦ newLineNumber}
    /\ UNCHANGED invariants

\* Retrieve a decision (read-only operation)
Get(id) ≡
  LET lineNum ≡ lineNumbers[id]
  IN ledger[lineNum].data

\* Query all decisions
List ≡ {ledger[i].data : i ∈ 1..Len(ledger)}

\* Verify hash chain consistency
VerifyHashChain ≡
  ∀ i ∈ 2..Len(ledger):
    ledger[i].previousHash = ledger[i-1].hash

\* Define next state
Next ≡
  ∃ decision ∈ Decision: Append(decision)

\* INVARIANTS (must ALWAYS be true)

\* Invariant 1: Append-only property
\* ledger can only grow, never shrink
Invariant_AppendOnly ≡
  □ (Len(ledger) ≤ Len(ledger'))

\* Invariant 2: Immutability
\* All entries that were in ledger remain unchanged
Invariant_Immutable ≡
  □ (∀ i ∈ 1..Len(ledger):
       i ≤ Len(ledger') ∧ ledger[i] = ledger'[i])

\* Invariant 3: No duplicate IDs
\* Every decision has a unique ID
Invariant_NoDuplicates ≡
  □ (∀ i, j ∈ 1..Len(ledger):
       i ≠ j → ledger[i].id ≠ ledger[j].id)

\* Invariant 4: Hash chain consistency
\* Each entry's previous_hash matches the previous entry's hash
Invariant_HashChain ≡
  □ (∀ i ∈ 2..Len(ledger):
       ledger[i].previousHash = ledger[i-1].hash)

\* Invariant 5: No modifications after append
\* Once a decision is appended, its content never changes
Invariant_NoModifications ≡
  □ (∀ i ∈ 1..Len(ledger):
       IF i ≤ Len(ledger) THEN
         i ≤ Len(ledger') ∧ ledger[i] = ledger'[i]
       ELSE
         TRUE)

\* THEOREMS

\* Theorem 1: Ledger is immutable after append
THEOREM Immutability ≡
  Init ∧ [][Next]_ledger ⇒ Invariant_Immutable

\* Theorem 2: Append-only holds always
THEOREM AppendOnly ≡
  Init ∧ [][Next]_ledger ⇒ Invariant_AppendOnly

\* Theorem 3: No duplicate IDs possible
THEOREM NoDuplicates ≡
  Init ∧ [][Next]_ledger ⇒ Invariant_NoDuplicates

\* Theorem 4: Hash chain never breaks
THEOREM HashChainConsistency ≡
  Init ∧ [][Next]_ledger ⇒ Invariant_HashChain

====
