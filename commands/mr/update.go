package mr

import (
	"github.com/lighttiger2505/lab/commands/internal"
	"github.com/lighttiger2505/lab/internal/api"
	gitlab "github.com/xanzy/go-gitlab"
)

type updateMethod struct {
	internal.Method
	client  api.MergeRequest
	opt     *CreateUpdateOption
	project string
	id      int
}

func (m *updateMethod) Process() (string, error) {
	// Getting exist merge request
	mergeRequest, err := m.client.GetMergeRequest(m.id, m.project)
	if err != nil {
		return "", err
	}

	// Create new title or description
	updatedTitle := mergeRequest.Title
	updatedMessage := mergeRequest.Description
	if m.opt.Title != "" {
		updatedTitle = m.opt.Title
	}
	if m.opt.Message != "" {
		updatedMessage = m.opt.Message
	}

	// Do update merge request
	_, err = m.client.UpdateMergeRequest(
		makeUpdateMergeRequestOption(m.opt, updatedTitle, updatedMessage),
		m.id,
		m.project,
	)
	if err != nil {
		return "", nil
	}

	// Return empty value
	return "", nil
}

type updateOnEditorMethod struct {
	internal.Method
	client   api.MergeRequest
	opt      *CreateUpdateOption
	project  string
	id       int
	editFunc func(program, file string) error
}

func (m *updateOnEditorMethod) Process() (string, error) {
	// Getting exist merge request
	mergeRequest, err := m.client.GetMergeRequest(m.id, m.project)
	if err != nil {
		return "", nil
	}

	// Starting editor for edit title and description
	title, message, err := internal.EditTitleAndDesc(
		"MERGE_REQUEST",
		internal.EditContents(mergeRequest.Title, mergeRequest.Description),
		m.editFunc,
	)
	if err != nil {
		return "", nil
	}

	// Do update merge request
	_, err = m.client.UpdateMergeRequest(
		makeUpdateMergeRequestOption(m.opt, title, message),
		m.id,
		m.project,
	)
	if err != nil {
		return "", nil
	}

	// Return empty value
	return "", nil
}

func makeUpdateMergeRequestOption(opt *CreateUpdateOption, title, description string) *gitlab.UpdateMergeRequestOptions {
	updateMergeRequestOptions := &gitlab.UpdateMergeRequestOptions{
		Title:        gitlab.String(title),
		Description:  gitlab.String(description),
		TargetBranch: gitlab.String(opt.TargetBranch),
	}
	if opt.StateEvent != "" {
		updateMergeRequestOptions.StateEvent = gitlab.String(opt.StateEvent)
	}
	if opt.AssigneeID != 0 {
		updateMergeRequestOptions.AssigneeID = gitlab.Int(opt.AssigneeID)
	}
	if opt.MilestoneID != 0 {
		updateMergeRequestOptions.MilestoneID = gitlab.Int(opt.MilestoneID)
	}
	ok, removeSourceBranchFlag := opt.RemoveSourceBranchFlag()
	if ok {
		updateMergeRequestOptions.RemoveSourceBranch = gitlab.Bool(removeSourceBranchFlag)
	}
	ok, squashFlag := opt.SquashFlag()
	if ok {
		updateMergeRequestOptions.Squash = gitlab.Bool(squashFlag)
	}

	return updateMergeRequestOptions
}
