---- MODULE homebase_TTrace_1785178088 ----
EXTENDS Sequences, TLCExt, Toolbox, homebase_TEConstants, Naturals, TLC, homebase

_expression ==
    LET homebase_TEExpression == INSTANCE homebase_TEExpression
    IN homebase_TEExpression!expression
----

_trace ==
    LET homebase_TETrace == INSTANCE homebase_TETrace
    IN homebase_TETrace!trace
----

_prop ==
    ~(([]<>(
            recoveryAttempts = (1)
            /\
            state = ("REPEAT")
            /\
            failed = (FALSE)
            /\
            recoveryLog = (<<1>>)
    ))/\([]<>(
            recoveryAttempts = (0)
            /\
            state = ("EXECUTE")
            /\
            failed = (FALSE)
            /\
            recoveryLog = (<<>>)
    )))
----

_init ==
    /\ state = _TETrace[1].state
    /\ recoveryAttempts = _TETrace[1].recoveryAttempts
    /\ failed = _TETrace[1].failed
    /\ recoveryLog = _TETrace[1].recoveryLog
----

_next ==
    /\ \E i,j \in DOMAIN _TETrace:
        /\ \/ /\ j = i + 1
              /\ i = TLCGet("level")
           \/ /\ i = _TTraceLassoEnd
              /\ j = _TTraceLassoStart
        /\ state  = _TETrace[i].state
        /\ state' = _TETrace[j].state
        /\ recoveryAttempts  = _TETrace[i].recoveryAttempts
        /\ recoveryAttempts' = _TETrace[j].recoveryAttempts
        /\ failed  = _TETrace[i].failed
        /\ failed' = _TETrace[j].failed
        /\ recoveryLog  = _TETrace[i].recoveryLog
        /\ recoveryLog' = _TETrace[j].recoveryLog

\* Uncomment the ASSUME below to write the states of the error trace
\* to the given file in Json format. Note that you can pass any tuple
\* to `JsonSerialize`. For example, a sub-sequence of _TETrace.
    \* ASSUME
    \*     LET J == INSTANCE Json
    \*         IN J!JsonSerialize("homebase_TTrace_1785178088.json", _TETrace)


_view ==
    <<state, recoveryAttempts, failed, recoveryLog, IF TLCGet("level") = _TTraceLassoEnd + 1 THEN _TTraceLassoStart ELSE TLCGet("level")>>
=============================================================================

 Note that you can extract this module `homebase_TEExpression`
  to a dedicated file to reuse `expression` (the module in the 
  dedicated `homebase_TEExpression.tla` file takes precedence 
  over the module `homebase_TEExpression` below).

---- MODULE homebase_TEExpression ----
EXTENDS Sequences, TLCExt, Toolbox, homebase_TEConstants, Naturals, TLC, homebase

expression == 
    [
        \* To hide variables of the `homebase` spec from the error trace,
        \* remove the variables below.  The trace will be written in the order
        \* of the fields of this record.
        state |-> state
        ,recoveryAttempts |-> recoveryAttempts
        ,failed |-> failed
        ,recoveryLog |-> recoveryLog
        
        \* Put additional constant-, state-, and action-level expressions here:
        \* ,_stateNumber |-> _TEPosition
        \* ,_stateUnchanged |-> state = state'
        
        \* Format the `state` variable as Json value.
        \* ,_stateJson |->
        \*     LET J == INSTANCE Json
        \*     IN J!ToJson(state)
        
        \* Lastly, you may build expressions over arbitrary sets of states by
        \* leveraging the _TETrace operator.  For example, this is how to
        \* count the number of times a spec variable changed up to the current
        \* state in the trace.
        \* ,_stateModCount |->
        \*     LET F[s \in DOMAIN _TETrace] ==
        \*         IF s = 1 THEN 0
        \*         ELSE IF _TETrace[s].state # _TETrace[s-1].state
        \*             THEN 1 + F[s-1] ELSE F[s-1]
        \*     IN F[_TEPosition - 1]
    ]

=============================================================================



Parsing and semantic processing can take forever if the trace below is long.
 In this case, it is advised to uncomment the module below to deserialize the
 trace from a generated binary file.

\*
\*---- MODULE homebase_TETrace ----
\*EXTENDS IOUtils, homebase_TEConstants, TLC, homebase
\*
\*trace == IODeserialize("homebase_TTrace_1785178088.bin", TRUE)
\*
\*=============================================================================
\*

---- MODULE homebase_TETrace ----
EXTENDS homebase_TEConstants, TLC, homebase

trace == 
    <<
    ([recoveryAttempts |-> 0,state |-> "PLAN",failed |-> FALSE,recoveryLog |-> <<>>]),
    ([recoveryAttempts |-> 0,state |-> "EXECUTE",failed |-> FALSE,recoveryLog |-> <<>>]),
    ([recoveryAttempts |-> 0,state |-> "RECOVER",failed |-> TRUE,recoveryLog |-> <<>>]),
    ([recoveryAttempts |-> 1,state |-> "REPEAT",failed |-> FALSE,recoveryLog |-> <<1>>])
    >>
----


=============================================================================

---- MODULE homebase_TEConstants ----
EXTENDS homebase

CONSTANTS _TTraceLassoStart, _TTraceLassoEnd

=============================================================================

---- CONFIG homebase_TTrace_1785178088 ----
CONSTANTS
    MaxRecoveryAttempts = 2
_TTraceLassoStart = 2
_TTraceLassoEnd = 4

PROPERTY
    _prop

CHECK_DEADLOCK
    \* CHECK_DEADLOCK off because of PROPERTY or INVARIANT above.
    FALSE

INIT
    _init

NEXT
    _next

VIEW
    _view

CONSTANT
    _TETrace <- _trace

ALIAS
    _expression
=============================================================================
\* Generated on Mon Jul 27 11:48:09 PDT 2026