module GraphMachine {
  datatype State = PLAN | EXECUTE | RECOVER | ESCALATE | REPEAT | COMPLETE
  
  class ExecutionContext {
    var state: State
    var recoveryAttempts: nat
    var humanApprovals: nat

    ghost predicate Valid()
      reads this
    {
      recoveryAttempts <= 2 && humanApprovals <= 3
    }

    constructor()
      ensures Valid()
      ensures state == PLAN
      ensures recoveryAttempts == 0
      ensures humanApprovals == 0
    {
      state := PLAN;
      recoveryAttempts := 0;
      humanApprovals := 0;
    }

    method StepPlan()
      requires Valid()
      requires state == PLAN
      modifies this
      ensures Valid()
      ensures state == EXECUTE
    {
      state := EXECUTE;
    }

    method StepExecute(success: bool)
      requires Valid()
      requires state == EXECUTE
      modifies this
      ensures Valid()
      ensures success ==> state == REPEAT
      ensures !success ==> state == RECOVER
    {
      if success {
        state := REPEAT;
      } else {
        state := RECOVER;
        recoveryAttempts := 0;
      }
    }

    method StepRecover(success: bool)
      requires Valid()
      requires state == RECOVER
      requires recoveryAttempts < 2
      modifies this
      ensures Valid()
      ensures success ==> state == EXECUTE
      ensures !success && old(recoveryAttempts) < 1 ==> state == RECOVER && recoveryAttempts == old(recoveryAttempts) + 1
      ensures !success && old(recoveryAttempts) == 1 ==> state == ESCALATE
    {
      recoveryAttempts := recoveryAttempts + 1;
      if success {
        state := EXECUTE;
      } else {
        if recoveryAttempts == 2 {
          state := ESCALATE;
        } else {
          state := RECOVER;
        }
      }
    }

    method StepEscalate(approved: bool)
      requires Valid()
      requires state == ESCALATE
      modifies this
      ensures Valid()
      ensures approved && old(humanApprovals) < 3 ==> state == EXECUTE && humanApprovals == old(humanApprovals) + 1
      ensures approved && old(humanApprovals) >= 3 ==> state == REPEAT && humanApprovals == old(humanApprovals)
      ensures !approved ==> state == COMPLETE && humanApprovals == old(humanApprovals)
    {
      if approved {
        if humanApprovals < 3 {
          humanApprovals := humanApprovals + 1;
          state := EXECUTE;
        } else {
          state := REPEAT;
        }
      } else {
        state := COMPLETE;
      }
    }

    method StepRepeat(moreWork: bool)
      requires Valid()
      requires state == REPEAT
      modifies this
      ensures Valid()
      ensures moreWork ==> state == EXECUTE
      ensures !moreWork ==> state == COMPLETE
    {
      if moreWork {
        state := EXECUTE;
      } else {
        state := COMPLETE;
      }
    }
  }
}
