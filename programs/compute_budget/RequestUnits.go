// Copyright 2021 github.com/gagliardetto
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package compute_budget

import (
	"encoding/binary"
	"errors"
	ag_format "github.com/desperatee/solana-go/text/format"
	ag_binary "github.com/gagliardetto/binary"
	ag_treeout "github.com/gagliardetto/treeout"
)

// RequestUnits
type RequestUnits struct {
	// Number of lamports to transfer to the new account
	Units *uint32
	// Number of lamports to transfer to the new account
	AdditionalFee *uint32
}

// NewTransferInstructionBuilder creates a new `Transfer` instruction builder.
func NewTransferInstructionBuilder() *RequestUnits {
	nd := &RequestUnits{}
	return nd
}

// Number of requested compute units
func (inst *RequestUnits) SetUnits(units uint32) *RequestUnits {
	inst.Units = &units
	return inst
}

// Additional fee in lamports
func (inst *RequestUnits) SetAdditionalFee(additionalFee uint32) *RequestUnits {
	inst.AdditionalFee = &additionalFee
	return inst
}

func (inst RequestUnits) Build() *Instruction {
	return &Instruction{BaseVariant: ag_binary.BaseVariant{
		Impl:   inst,
		TypeID: ag_binary.TypeIDFromUint32(Instruction_RequestUnits, binary.LittleEndian),
	}}
}

// ValidateAndBuild validates the instruction parameters and accounts;
// if there is a validation error, it returns the error.
// Otherwise, it builds and returns the instruction.
func (inst RequestUnits) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

func (inst *RequestUnits) Validate() error {
	// Check whether all (required) parameters are set:
	{
		if inst.Units == nil {
			return errors.New("Units parameter is not set")
		}
		if inst.AdditionalFee == nil {
			return errors.New("AdditionalFee parameter is not set")
		}
	}
	return nil
}

func (inst *RequestUnits) EncodeToTree(parent ag_treeout.Branches) {
	parent.Child(ag_format.Program(ProgramName, ProgramID)).
		//
		ParentFunc(func(programBranch ag_treeout.Branches) {
			programBranch.Child(ag_format.Instruction("RequestUnits")).
				//
				ParentFunc(func(instructionBranch ag_treeout.Branches) {

					// Parameters of the instruction:
					instructionBranch.Child("Params").ParentFunc(func(paramsBranch ag_treeout.Branches) {
						paramsBranch.Child(ag_format.Param("Units", *inst.Units))
						paramsBranch.Child(ag_format.Param("AdditionalFee", *inst.AdditionalFee))
					})
				})
		})
}

func (inst RequestUnits) MarshalWithEncoder(encoder *ag_binary.Encoder) error {
	// Serialize `RequestedComputeUnits` param:
	{
		err := encoder.Encode(*inst.Units)
		if err != nil {
			return err
		}
	}
	// Serialize `AdditionalFee` param:
	{
		err := encoder.Encode(*inst.AdditionalFee)
		if err != nil {
			return err
		}
	}
	return nil
}

func (inst *RequestUnits) UnmarshalWithDecoder(decoder *ag_binary.Decoder) error {
	// Deserialize `RequestedComputeUnits` param:
	{
		err := decoder.Decode(&inst.Units)
		if err != nil {
			return err
		}
	}
	// Deserialize `AdditionalFee` param:
	{
		err := decoder.Decode(&inst.AdditionalFee)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewRequestUnitsInstruction declares a new RequestUnits instruction with the provided parameters and accounts.
func NewRequestUnitsInstruction(
	// Parameters:
	requestedComputeUnits uint32,
	additionalFee uint32) *RequestUnits {
	return NewTransferInstructionBuilder().
		SetUnits(requestedComputeUnits).
		SetAdditionalFee(additionalFee)
}
