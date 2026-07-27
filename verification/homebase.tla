--------------------------- MODULE homebase ---------------------------
EXTENDS Integers, Sequences, TLC

CONSTANTS 
    MaxRecoveryAttempts,
    MaxHumanApprovals,
    MaxDecisions

VARIABLES 
    state,
    recoveryAttempts,
    recoveryLog,    \* AWS pattern: explicit logging of all attempts
    humanApprovals,
    failed,
    remainingDecisions

vars == <<state, recoveryAttempts, recoveryLog, humanApprovals, failed, remainingDecisions>>

Init == 
    /\ state = "PLAN"
    /\ recoveryAttempts = 0
    /\ recoveryLog = <<>>
    /\ humanApprovals = 0
    /\ failed = FALSE
    /\ remainingDecisions = MaxDecisions

PlanNode == 
    /\ state = "PLAN"
    /\ state' = "EXECUTE"
    /\ UNCHANGED <<recoveryAttempts, recoveryLog, humanApprovals, failed, remainingDecisions>>

ExecuteNode == 
    /\ state = "EXECUTE"
    /\ \/ /\ failed' = FALSE
          /\ state' = "REPEAT"
       \/ /\ failed' = TRUE
          /\ IF recoveryAttempts < MaxRecoveryAttempts 
             THEN state' = "RECOVER" 
             ELSE state' = "ESCALATE"
    /\ UNCHANGED <<recoveryAttempts, recoveryLog, humanApprovals, remainingDecisions>>

RecoverNode == 
    /\ state = "RECOVER"
    /\ recoveryAttempts < MaxRecoveryAttempts
    /\ recoveryAttempts' = recoveryAttempts + 1
    /\ recoveryLog' = Append(recoveryLog, recoveryAttempts') \* AWS Pattern: log each attempt
    /\ \/ /\ failed' = FALSE
          /\ state' = "EXECUTE" \* FIX: Go back to EXECUTE to actually record it
       \/ /\ failed' = TRUE
          /\ IF recoveryAttempts' < MaxRecoveryAttempts 
             THEN state' = "RECOVER" 
             ELSE state' = "ESCALATE"
    /\ UNCHANGED <<humanApprovals, remainingDecisions>>

EscalateNode == 
    /\ state = "ESCALATE"
    /\ \/ /\ state' = "EXECUTE"  \* Human approves
          /\ humanApprovals < MaxHumanApprovals
          /\ humanApprovals' = humanApprovals + 1
          /\ recoveryAttempts' = 0 \* Reset for the approved retry
          /\ recoveryLog' = <<>>
          /\ UNCHANGED <<remainingDecisions>>
       \/ /\ state' = "REPEAT" \* Human rejects or exceeded MaxHumanApprovals
          /\ UNCHANGED <<recoveryAttempts, recoveryLog, humanApprovals, remainingDecisions>>
    /\ UNCHANGED <<failed>>

RepeatNode ==
    /\ state = "REPEAT"
    /\ \/ /\ remainingDecisions > 0
          /\ state' = "EXECUTE" \* More decisions
          /\ remainingDecisions' = remainingDecisions - 1
          /\ recoveryAttempts' = 0
          /\ recoveryLog' = <<>>
          /\ humanApprovals' = 0
       \/ /\ remainingDecisions = 0
          /\ state' = "COMPLETE" \* No more decisions
          /\ UNCHANGED <<recoveryAttempts, recoveryLog, humanApprovals, remainingDecisions>>
    /\ UNCHANGED <<failed>>

CompleteNode == 
    /\ state = "COMPLETE"
    /\ state' = "DONE"
    /\ UNCHANGED <<recoveryAttempts, recoveryLog, humanApprovals, failed, remainingDecisions>>

Next == 
    \/ PlanNode 
    \/ ExecuteNode 
    \/ RecoverNode 
    \/ EscalateNode 
    \/ RepeatNode
    \/ CompleteNode
    \/ (state = "DONE" /\ UNCHANGED vars)

Spec == Init /\ [][Next]_vars /\ WF_vars(Next)

-----------------------------------------------------------------------------
TypeOK == 
    /\ state \in {"PLAN", "EXECUTE", "RECOVER", "ESCALATE", "REPEAT", "COMPLETE", "DONE"}
    /\ recoveryAttempts \in 0..MaxRecoveryAttempts
    /\ humanApprovals \in 0..MaxHumanApprovals

\* INVARIANT 1: Strict escalation limit
BoundedRecoveryInvariant == 
    recoveryAttempts <= MaxRecoveryAttempts

\* INVARIANT 2: AWS Pattern - Every attempt must be logged
RecoveryLoggingInvariant ==
    Len(recoveryLog) = recoveryAttempts

\* PROPERTY: No Infinite Retries (eventually we make progress)
Termination == 
    <>(state = "DONE")

=============================================================================
